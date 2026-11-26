// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package avo

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"sync"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile"
)

// ViewBadge is shown on the right side of an anyprofile-view window titlebar.
type ViewBadge = mobile.ViewBadge

// ViewInputEvent is emitted by a Master anyprofile-view window during Sync.
type ViewInputEvent = mobile.ViewInputEvent

type viewEvent struct {
	Ev    string  `json:"ev"`
	Muted bool    `json:"muted,omitempty"`
	Phase string  `json:"phase,omitempty"`
	RX    float64 `json:"rx,omitempty"`
	RY    float64 `json:"ry,omitempty"`
	DY    float64 `json:"dy,omitempty"`
	Code  int     `json:"code,omitempty"`
	Down  bool    `json:"down,omitempty"`
	Text  string  `json:"text,omitempty"`
}

type syncRoleState struct {
	role  string
	count int
}

// viewIPC owns the per-window Unix socket server used to send zero-flicker
// commands to anyprofile-view and receive focus/input events back.
type viewIPC struct {
	path   string
	ln     net.Listener
	mu     sync.Mutex
	conn   net.Conn
	closed bool
}

func startViewIPC(path string, onConnect func(*viewIPC), onEvent func(viewEvent)) (*viewIPC, error) {
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	v := &viewIPC{path: path, ln: ln}
	go v.acceptLoop(onConnect, onEvent)
	return v, nil
}

func (v *viewIPC) acceptLoop(onConnect func(*viewIPC), onEvent func(viewEvent)) {
	for {
		c, err := v.ln.Accept()
		if err != nil {
			return
		}
		v.mu.Lock()
		if v.closed {
			v.mu.Unlock()
			_ = c.Close()
			return
		}
		if v.conn != nil {
			_ = v.conn.Close()
		}
		v.conn = c
		v.mu.Unlock()

		if onConnect != nil {
			onConnect(v)
		}
		v.readLoop(c, onEvent)
	}
}

func (v *viewIPC) readLoop(c net.Conn, onEvent func(viewEvent)) {
	defer func() {
		v.mu.Lock()
		if v.conn == c {
			v.conn = nil
		}
		v.mu.Unlock()
		_ = c.Close()
	}()
	sc := bufio.NewScanner(c)
	sc.Buffer(make([]byte, 4096), 64*1024)
	for sc.Scan() {
		var ev viewEvent
		if json.Unmarshal(sc.Bytes(), &ev) == nil && ev.Ev != "" && onEvent != nil {
			onEvent(ev)
		}
	}
}

func (v *viewIPC) send(msg any) bool {
	if v == nil {
		return false
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return false
	}
	b = append(b, '\n')
	v.mu.Lock()
	c := v.conn
	if c == nil || v.closed {
		v.mu.Unlock()
		return false
	}
	_ = c.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
	_, err = c.Write(b)
	v.mu.Unlock()
	return err == nil
}

func (v *viewIPC) close() {
	if v == nil {
		return
	}
	v.mu.Lock()
	v.closed = true
	if v.conn != nil {
		_ = v.conn.Close()
		v.conn = nil
	}
	ln := v.ln
	v.mu.Unlock()
	if ln != nil {
		_ = ln.Close()
	}
	_ = os.Remove(v.path)
}

// SetFocusAudio enables or disables MuMu-style auto-mute for background phones.
func (d *Driver) SetFocusAudio(on bool) {
	d.mu.Lock()
	d.focusAudio = on
	d.mu.Unlock()
}

// FocusAudio reports whether background phones auto-mute when another phone focuses.
func (d *Driver) FocusAudio() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.focusAudio
}

// SetEcoFPS switches all open phone viewports between Normal (uncapped/15fps)
// and Eco (30fps/10fps) frame rate limits immediately.
func (d *Driver) SetEcoFPS(eco bool) {
	d.mu.Lock()
	d.ecoMode = eco
	fg, bg := d.fpsLimitsLocked()
	var ipcs []*viewIPC
	for _, p := range d.procs {
		if p.ipc != nil {
			ipcs = append(ipcs, p.ipc)
		}
	}
	d.mu.Unlock()
	cmd := map[string]any{"cmd": "set-fps", "fg": fg, "bg": bg}
	for _, ipc := range ipcs {
		ipc.send(cmd)
	}
}

func (d *Driver) fpsLimitsLocked() (fg, bg int) {
	if d.ecoMode {
		return 30, 10
	}
	return 0, 15
}

// SetBadge updates the proxy/country status badge shown in the phone's titlebar.
func (d *Driver) SetBadge(deviceID string, b ViewBadge) {
	d.mu.Lock()
	if d.badges == nil {
		d.badges = make(map[string]ViewBadge)
	}
	d.badges[deviceID] = b
	var ipc *viewIPC
	if p := d.procs[deviceID]; p != nil {
		ipc = p.ipc
	}
	d.mu.Unlock()
	if ipc != nil {
		ipc.send(map[string]any{
			"cmd":     "badge",
			"country": b.Country,
			"status":  b.Status,
			"label":   b.Label,
		})
	}
}

// SetSyncRole marks a phone window as "master", "follower", or "none" so the
// Master viewport forwards live touch/key input and displays a sync pill.
func (d *Driver) SetSyncRole(deviceID, role string, count int) {
	d.mu.Lock()
	if d.syncRoles == nil {
		d.syncRoles = make(map[string]syncRoleState)
	}
	if role == "" || role == "none" {
		delete(d.syncRoles, deviceID)
		role = "none"
	} else {
		d.syncRoles[deviceID] = syncRoleState{role: role, count: count}
	}
	var ipc *viewIPC
	if p := d.procs[deviceID]; p != nil {
		ipc = p.ipc
	}
	d.mu.Unlock()
	if ipc != nil {
		ipc.send(map[string]any{"cmd": "sync-role", "role": role, "count": count})
	}
}

// SetMasterInputHandler registers the callback invoked whenever a Master
// viewport emits a touch, scroll, key, or text event during Sync.
func (d *Driver) SetMasterInputHandler(fn func(deviceID string, ev ViewInputEvent)) {
	d.mu.Lock()
	d.onMasterInput = fn
	d.mu.Unlock()
}

// SendControlCommands sends one or more raw avo-vmm control.sock commands
// (e.g. "touch 0 1:16000:16000", "key 158 1") directly to a running VM.
func (d *Driver) SendControlCommands(ctx context.Context, deviceID string, cmds ...string) error {
	p := d.proc(deviceID)
	if p == nil {
		return os.ErrNotExist
	}
	for _, cmd := range cmds {
		if _, err := sendControl(ctx, p.ctrlSock, cmd); err != nil {
			return err
		}
	}
	return nil
}

// ToggleAllWindowsVisible implements the Boss Key (Cmd+Shift+B): hides or shows
// every open phone window instantly without stopping the underlying VMs.
func (d *Driver) ToggleAllWindowsVisible() bool {
	d.mu.Lock()
	d.windowsHidden = !d.windowsHidden
	visible := !d.windowsHidden
	var ipcs []*viewIPC
	for _, p := range d.procs {
		if p.ipc != nil {
			ipcs = append(ipcs, p.ipc)
		}
	}
	d.mu.Unlock()
	cmd := map[string]any{"cmd": "set-visible", "visible": visible}
	for _, ipc := range ipcs {
		ipc.send(cmd)
	}
	return visible
}

// ToggleAllMuted implements the global Mute-All hotkey (Cmd+Shift+M): mutes
// every running phone if any is unmuted, otherwise unmutes them all.
func (d *Driver) ToggleAllMuted(ctx context.Context) bool {
	d.mu.Lock()
	type item struct {
		id  string
		p   *proc
		ipc *viewIPC
	}
	var items []item
	anyUnmuted := false
	for id, p := range d.procs {
		items = append(items, item{id: id, p: p, ipc: p.ipc})
		if !d.userMuted[id] {
			anyUnmuted = true
		}
	}
	targetMute := anyUnmuted
	if d.userMuted == nil {
		d.userMuted = make(map[string]bool)
	}
	for _, it := range items {
		d.userMuted[it.id] = targetMute
	}
	d.mu.Unlock()

	vol := "10"
	if targetMute {
		vol = "0"
	}
	for _, it := range items {
		go func(it item) {
			c, cancel := context.WithTimeout(ctx, 4*time.Second)
			defer cancel()
			_, _ = it.p.adb.Shell(c, "cmd media_session volume --stream 3 --set "+vol)
			if it.ipc != nil {
				it.ipc.send(map[string]any{"cmd": "set-muted", "muted": targetMute})
			}
		}(it)
	}
	return targetMute
}

func (d *Driver) handleViewEvent(devID string, ev viewEvent) {
	switch ev.Ev {
	case "mute-toggle":
		d.mu.Lock()
		if d.userMuted == nil {
			d.userMuted = make(map[string]bool)
		}
		d.userMuted[devID] = ev.Muted
		d.mu.Unlock()
	case "focus":
		d.applyFocusAudio(devID)
	case "touch", "scroll", "key", "text":
		d.mu.Lock()
		fn := d.onMasterInput
		d.mu.Unlock()
		if fn != nil {
			fn(devID, ViewInputEvent{
				Kind:  ev.Ev,
				Phase: ev.Phase,
				RX:    ev.RX,
				RY:    ev.RY,
				DY:    ev.DY,
				Code:  ev.Code,
				Down:  ev.Down,
				Text:  ev.Text,
			})
		}
	}
}

func (d *Driver) applyFocusAudio(focusedID string) {
	d.mu.Lock()
	if !d.focusAudio || len(d.procs) < 2 {
		d.mu.Unlock()
		return
	}
	type target struct {
		p     *proc
		ipc   *viewIPC
		muted bool
	}
	var targets []target
	for id, p := range d.procs {
		shouldMute := id != focusedID || d.userMuted[id]
		targets = append(targets, target{p: p, ipc: p.ipc, muted: shouldMute})
	}
	d.mu.Unlock()

	for _, t := range targets {
		go func(t target) {
			c, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			vol := "10"
			if t.muted {
				vol = "0"
			}
			_, _ = t.p.adb.Shell(c, "cmd media_session volume --stream 3 --set "+vol)
			if t.ipc != nil {
				t.ipc.send(map[string]any{"cmd": "set-muted", "muted": t.muted})
			}
		}(t)
	}
}
