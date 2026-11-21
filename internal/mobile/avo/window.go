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

func (d *Driver) openWindowsLocked() int {
	n := 0
	for _, p := range d.procs {
		if p.view != nil {
			n++
		}
	}
	return n
}

func (d *Driver) showOn(ctx context.Context, dev mobile.Device, p *proc) {
	d.mu.Lock()
	if p.view != nil {
		d.mu.Unlock()
		return
	}
	n := d.openWindowsLocked()
	d.mu.Unlock()

	saved, ok := d.loadWindow(dev)
	pos := saved
	if !ok {
		pos = vm.DefaultWindow(n, dev.ScreenW, dev.ScreenH)
		pos.OnTop, pos.Muted = saved.OnTop, saved.Muted
	}
	title := dev.Name
	if title == "" {
		title = "AnyProfile Mobile"
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
	}
	if pos.OnTop {
		args = append(args, "--on-top")
	}
	if pos.Muted {
		args = append(args, "--muted")
	}

	cmd := exec.Command(p.rt.View, args...)
	cmd.Env = p.adb.Env()
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		d.log.Warn("start anyprofile-view", "device", dev.ID, "err", err)
		return
	}
	done := make(chan struct{})
	d.mu.Lock()
	p.view = cmd
	p.viewDone = done
	d.mu.Unlock()
	go func() {
		_ = cmd.Wait()
		close(done)
		d.mu.Lock()
		if p.view == cmd {
			p.view = nil
			p.viewDone = nil
		}
		d.mu.Unlock()
	}()
	_ = ctx
}

func (d *Driver) closeWindow(p *proc) {
	d.mu.Lock()
	cmd := p.view
	done := p.viewDone
	d.mu.Unlock()
	if cmd == nil {
		return
	}
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	if done != nil {
		<-done
	}
}

// Show opens the device's Metal viewport window after Stop/Headless or close.
func (d *Driver) Show(ctx context.Context, dev mobile.Device) error {
	p := d.proc(dev.ID)
	if p == nil {
		return errors.New("the device is not running")
	}
	if p.paused {
		if err := d.Resume(dev.ID); err != nil {
			return err
		}
	}
	d.showOn(ctx, dev, p)
	return nil
}

// Windowed reports whether the device currently has an open viewport window.
func (d *Driver) Windowed(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	p := d.procs[id]
	return p != nil && p.view != nil
}

// SetWindowOnTop stores the always-on-top flag and reopens an open viewport.
func (d *Driver) SetWindowOnTop(ctx context.Context, dev mobile.Device, on bool) error {
	s, ok := d.loadWindow(dev)
	if !ok {
		s = vm.DefaultWindow(0, dev.ScreenW, dev.ScreenH)
	}
	s.OnTop = on
	if err := d.saveWindow(dev, s); err != nil {
		return err
	}
	if p := d.proc(dev.ID); p != nil && p.view != nil {
		d.closeWindow(p)
		d.showOn(ctx, dev, p)
	}
	return nil
}

// WindowOnTop reports the stored always-on-top flag.
func (d *Driver) WindowOnTop(dev mobile.Device) bool {
	s, _ := d.loadWindow(dev)
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
	if p := d.proc(dev.ID); p != nil {
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		vol := "10"
		if muted {
			vol = "0"
		}
		_, _ = p.adb.Shell(c, "cmd media_session volume --stream 3 --set "+vol)
		if p.view != nil {
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
		prev, _ := d.loadWindow(dev)
		s := slots[i]
		s.OnTop, s.Muted = prev.OnTop, prev.Muted
		_ = d.saveWindow(dev, s)
		if p := d.proc(dev.ID); p != nil {
			d.closeWindow(p)
			d.showOn(ctx, dev, p)
		}
	}
	return len(open)
}
