// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package avo

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/vm"
)

func (d *Driver) windowFile(dev mobile.Device) string {
	return filepath.Join(d.dir(dev), "window.json")
}

func (d *Driver) loadWindow(dev mobile.Device) (vm.WindowState, bool) {
	b, err := os.ReadFile(d.windowFile(dev))
	if err != nil {
		return vm.WindowState{}, false
	}
	var s vm.WindowState
	if json.Unmarshal(b, &s) != nil || s.W < 200 || s.H < 300 {
		return vm.WindowState{}, false
	}
	s.OnTop = !s.Unpinned
	if a, ok := vm.ScreenWorkArea(); ok {
		if s.X+s.W < a.X+40 || s.X > a.X+a.W-40 || s.Y+s.H < a.Y+40 || s.Y > a.Y+a.H-40 {
			return vm.WindowState{}, false
		}
	}
	return s, true
}

func (d *Driver) saveWindow(dev mobile.Device, s vm.WindowState) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := d.windowFile(dev) + ".part"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, d.windowFile(dev))
}

func rectsOverlap(a, b vm.WindowState) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}

// nextCleanSlotLocked picks the first left-to-right MuMu slot that does not
// overlap any other currently open or booting phone window.
func (d *Driver) nextCleanSlotLocked(excludeID string, sw, sh int) (vm.WindowState, bool) {
	if len(d.views) == 0 {
		return vm.WindowState{}, false
	}
	var others []vm.WindowState
	for id, r := range d.views {
		if id == excludeID {
			continue
		}
		if s, ok := d.loadWindow(mobile.Device{ID: id}); ok && !s.Minimized {
			r = s
		}
		others = append(others, r)
	}
	if len(others) == 0 {
		return vm.WindowState{}, false
	}
	for slot := 0; slot < len(others)+4; slot++ {
		cand := vm.DefaultWindow(slot, sw, sh)
		clash := false
		for _, o := range others {
			if rectsOverlap(cand, o) {
				clash = true
				break
			}
		}
		if !clash {
			return cand, true
		}
	}
	return vm.DefaultWindow(len(others), sw, sh), true
}

func (d *Driver) showOn(ctx context.Context, dev mobile.Device, p *proc) {
	d.showOnAt(ctx, dev, p, nil)
}

func (d *Driver) showOnAt(ctx context.Context, dev mobile.Device, p *proc, forced *vm.WindowState) {
	title := dev.Name
	if title == "" {
		title = "AnyProfile Mobile"
	}
	d.mu.Lock()
	if d.views == nil {
		d.views = make(map[string]vm.WindowState)
	}
	if p.view != nil {
		if forced == nil && p.viewAppMode == dev.AppMode {
			if p.viewTitle != title {
				if p.ipc != nil && p.ipc.send(map[string]any{"cmd": "set-title", "title": title}) {
					p.viewTitle = title
				}
			}
			if p.viewTitle == title {
				if dev.NoFocus {
					d.mu.Unlock()
					return
				}
				saved, ok := d.loadWindow(dev)
				if (!ok || !saved.Minimized) && p.view.Process != nil && activateView(p.view.Process.Pid) {
					d.mu.Unlock()
					return
				}
			}
		}
		d.mu.Unlock()
		d.closeWindow(p)
		d.mu.Lock()
	}

	saved, ok := d.loadWindow(dev)
	var pos vm.WindowState
	switch {
	case forced != nil:
		pos = *forced
	default:
		if next, hasOthers := d.nextCleanSlotLocked(dev.ID, dev.ScreenW, dev.ScreenH); hasOthers {
			// When other phones are already open, snap cleanly into the next free
			// MuMu slot so windows line up side-by-side instead of piling up.
			pos = next
			if ok {
				pos.OnTop, pos.Unpinned, pos.Muted = saved.OnTop, saved.Unpinned, saved.Muted
			}
		} else if ok {
			pos = saved
		} else {
			pos = vm.DefaultWindow(0, dev.ScreenW, dev.ScreenH)
		}
	}
	// Reserve this slot immediately under d.mu so concurrent device boots
	// never claim the same coordinates.
	d.views[dev.ID] = pos
	d.mu.Unlock()

	if saved.Minimized {
		saved.Minimized = false
		_ = d.saveWindow(dev, saved)
	}
	var ipc *viewIPC
	viewSock := ""
	if p.rtDir != "" {
		viewSock = filepath.Join(p.rtDir, "view.sock")
		if v, err := startViewIPC(viewSock, func(v *viewIPC) {
			d.mu.Lock()
			fg, bg := d.fpsLimitsLocked()
			badge, hasBadge := d.badges[dev.ID]
			sr, hasSync := d.syncRoles[dev.ID]
			d.mu.Unlock()
			v.send(map[string]any{"cmd": "set-fps", "fg": fg, "bg": bg})
			if hasBadge {
				v.send(map[string]any{"cmd": "badge", "country": badge.Country, "status": badge.Status, "label": badge.Label})
			}
			if hasSync {
				v.send(map[string]any{"cmd": "sync-role", "role": sr.role, "count": sr.count})
			}
		}, func(ev viewEvent) {
			d.handleViewEvent(dev.ID, ev)
		}); err == nil {
			ipc = v
		}
	}

	args := []string{
		"--control", p.ctrlSock,
		"--title", title,
		"--x", strconv.Itoa(pos.X),
		"--y", strconv.Itoa(pos.Y),
		"--w", strconv.Itoa(pos.W),
		"--h", strconv.Itoa(pos.H),
		"--bounds-file", d.windowFile(dev),
		"--adb-bin", p.adb.Bin,
		"--adb-serial", p.adb.Serial(),
		"--owner-pid", strconv.Itoa(os.Getpid()),
	}
	if ipc != nil && viewSock != "" {
		args = append(args, "--view-sock", viewSock)
	}
	if pos.OnTop {
		args = append(args, "--on-top")
	}
	if pos.Muted {
		args = append(args, "--muted")
	}
	if dev.AppMode {
		args = append(args, "--app-mode")
	}
	if dev.NoFocus {
		args = append(args, "--no-focus")
	}

	cmd := exec.Command(p.rt.View, args...)
	cmd.Env = p.adb.Env()
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		ipc.close()
		d.mu.Lock()
		delete(d.views, dev.ID)
		d.mu.Unlock()
		d.log.Warn("start anyprofile-view", "device", dev.ID, "err", err)
		return
	}
	done := make(chan struct{})
	d.mu.Lock()
	p.view = cmd
	p.viewDone = done
	p.viewTitle = title
	p.viewAppMode = dev.AppMode
	p.ipc = ipc
	d.mu.Unlock()
	go func() {
		_ = cmd.Wait()
		ipc.close()
		close(done)
		d.mu.Lock()
		if p.view == cmd {
			p.view = nil
			p.viewDone = nil
			p.viewTitle = ""
			p.viewAppMode = false
			p.ipc = nil
			delete(d.views, dev.ID)
		}
		d.mu.Unlock()
	}()
	_ = ctx
}

func (d *Driver) closeWindow(p *proc) {
	d.mu.Lock()
	cmd := p.view
	done := p.viewDone
	ipc := p.ipc
	p.ipc = nil
	d.mu.Unlock()
	ipc.close()
	if cmd == nil {
		return
	}
	if cmd.Process != nil {
		// Ask cleanly first so anyprofile-view can hand focus back on the same desktop.
		_ = cmd.Process.Signal(syscall.SIGTERM)
		if done != nil {
			select {
			case <-done:
				return
			case <-time.After(400 * time.Millisecond):
			}
		}
		_ = cmd.Process.Kill()
	}
	if done != nil {
		<-done
	}
}

// Hide closes the device's viewport window without stopping the VM.
func (d *Driver) Hide(id string) {
	if p := d.proc(id); p != nil {
		d.closeWindow(p)
	}
}

// Show opens the device's Metal viewport window after Stop/Headless or close.
func (d *Driver) Show(ctx context.Context, dev mobile.Device) error {
	p := d.proc(dev.ID)
	if p == nil {
		return errors.New("the device is not running")
	}
	// resume is idempotent; always thaw in case anyprofile-view paused the VMM on close.
	if err := d.Resume(dev.ID); err != nil {
		return err
	}
	// Headless-booted or Doze-idled Android sleeps the display (mWakefulness=Asleep → black frames).
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	_, _ = p.adb.Shell(c, "dumpsys deviceidle unforce >/dev/null 2>&1 || true; settings put system screen_off_timeout 2147483647; svc power stayon true; input keyevent 224; wm dismiss-keyguard")
	cancel()
	d.showOn(ctx, dev, p)
	return nil
}

// Windowed reports whether the device currently has a visible (non-minimized) viewport window.
func (d *Driver) Windowed(id string) bool {
	d.mu.Lock()
	p := d.procs[id]
	hasView := p != nil && p.view != nil
	d.mu.Unlock()
	if !hasView {
		return false
	}
	s, ok := d.loadWindow(mobile.Device{ID: id})
	return !ok || !s.Minimized
}

// SetWindowOnTop stores the always-on-top flag and applies it live via IPC
// without reopening the window (falling back to reopen on older view binaries).
func (d *Driver) SetWindowOnTop(ctx context.Context, dev mobile.Device, on bool) error {
	s, ok := d.loadWindow(dev)
	if !ok {
		s = vm.DefaultWindow(0, dev.ScreenW, dev.ScreenH)
	}
	s.OnTop = on
	s.Unpinned = !on
	if err := d.saveWindow(dev, s); err != nil {
		return err
	}
	if p := d.proc(dev.ID); p != nil && p.view != nil {
		if p.ipc != nil && p.ipc.send(map[string]any{"cmd": "set-on-top", "on": on}) {
			return nil
		}
		dev.NoFocus = true
		d.closeWindow(p)
		d.showOn(ctx, dev, p)
	}
	return nil
}

// WindowOnTop reports the stored always-on-top flag (defaults to true).
func (d *Driver) WindowOnTop(dev mobile.Device) bool {
	s, ok := d.loadWindow(dev)
	if !ok {
		return true
	}
	return s.OnTop
}

// SetWindowMuted stores the mute preference and applies it live when running.
func (d *Driver) SetWindowMuted(ctx context.Context, dev mobile.Device, muted bool) error {
	s, ok := d.loadWindow(dev)
	if !ok {
		s = vm.DefaultWindow(0, dev.ScreenW, dev.ScreenH)
	}
	s.Muted = muted
	if err := d.saveWindow(dev, s); err != nil {
		return err
	}
	d.mu.Lock()
	if d.userMuted == nil {
		d.userMuted = make(map[string]bool)
	}
	d.userMuted[dev.ID] = muted
	d.mu.Unlock()
	if p := d.proc(dev.ID); p != nil {
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		vol := "10"
		if muted {
			vol = "0"
		}
		_, _ = p.adb.Shell(c, "cmd media_session volume --stream 3 --set "+vol)
		if p.view != nil {
			if p.ipc != nil && p.ipc.send(map[string]any{"cmd": "set-muted", "muted": muted}) {
				return nil
			}
			dev.NoFocus = true
			d.closeWindow(p)
			d.showOn(ctx, dev, p)
		}
	}
	return nil
}

// SetLocation updates the guest hardware GNSS simulator via control.sock.
func (d *Driver) SetLocation(ctx context.Context, id string, lat, lon float64) {
	if p := d.proc(id); p != nil {
		_, _ = sendControl(ctx, p.ctrlSock, "location "+strconv.FormatFloat(lat, 'f', 6, 64)+" "+strconv.FormatFloat(lon, 'f', 6, 64))
	}
}

// WindowMuted reports the stored mute preference.
func (d *Driver) WindowMuted(dev mobile.Device) bool {
	s, _ := d.loadWindow(dev)
	return s.Muted
}

// ArrangeWindows repositions every open anyprofile-view window in devs.
// When view.sock IPC is connected, windows move in a single frame with zero flicker.
func (d *Driver) ArrangeWindows(ctx context.Context, layout string, devs []mobile.Device) int {
	a, ok := vm.ScreenWorkArea()
	if !ok {
		a = vm.WindowRect{X: 0, Y: 30, W: 1440, H: 840}
	}
	var open []mobile.Device
	for _, dev := range devs {
		if p := d.proc(dev.ID); p != nil && p.view != nil {
			open = append(open, dev)
		}
	}
	if len(open) == 0 {
		return 0
	}
	slots := vm.ArrangeLayout(a, layout, len(open), open[0].ScreenW, open[0].ScreenH)
	for i, dev := range open {
		prev, ok := d.loadWindow(dev)
		s := slots[i]
		if ok {
			s.OnTop, s.Unpinned, s.Muted = prev.OnTop, prev.Unpinned, prev.Muted
		} else {
			s.OnTop = true
		}
		_ = d.saveWindow(dev, s)
		if p := d.proc(dev.ID); p != nil {
			d.mu.Lock()
			if d.views == nil {
				d.views = make(map[string]vm.WindowState)
			}
			d.views[dev.ID] = s
			ipc := p.ipc
			d.mu.Unlock()
			if ipc != nil && ipc.send(map[string]any{
				"cmd": "set-frame",
				"x":   s.X,
				"y":   s.Y,
				"w":   s.W,
				"h":   s.H,
			}) {
				continue
			}
			dev.NoFocus = true
			d.closeWindow(p)
			d.showOnAt(ctx, dev, p, &s)
		}
	}
	return len(open)
}
