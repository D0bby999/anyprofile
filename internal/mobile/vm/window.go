// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package vm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/D0bby999/anyprofile/internal/mobile"
)

// windowState is where a device's scrcpy window was last seen, in screen
// points from the top left, plus its always-on-top and audio mute flags.
type windowState struct {
	X     int  `json:"x"`
	Y     int  `json:"y"`
	W     int  `json:"w"`
	H     int  `json:"h"`
	OnTop bool `json:"onTop"`
	Muted bool `json:"muted,omitempty"`
}

func (v *Driver) windowFile(d mobile.Device) string {
	return filepath.Join(v.dir(d), "window.json")
}

func (v *Driver) loadWindow(d mobile.Device) (windowState, bool) {
	b, err := os.ReadFile(v.windowFile(d))
	if err != nil {
		return windowState{}, false
	}
	var s windowState
	if json.Unmarshal(b, &s) != nil || s.W < 200 || s.H < 300 {
		return windowState{}, false
	}
	// A window from another monitor arrangement can be off the screen.
	if a, ok := screenWorkArea(); ok {
		if s.X+s.W < a.X+40 || s.X > a.X+a.W-40 || s.Y+s.H < a.Y+40 || s.Y > a.Y+a.H-40 {
			return windowState{}, false
		}
	}
	return s, true
}

func (v *Driver) saveWindow(d mobile.Device, s windowState) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := v.windowFile(d) + ".part"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, v.windowFile(d))
}

const (
	// windowEdge is the gap kept from the right edge of the screen.
	windowEdge = 28
	// windowGap is the strip between two phones standing side by side.
	windowGap = 8
	// windowCascade is how far a phone that does not fit across the screen
	// steps from the one below it, so it is still visibly a separate window.
	windowCascade = 24
)

// defaultWindow puts a phone-shaped window at the right edge of the screen,
// clear of the Dock and the menu bar. n is how many device windows are already
// open: a phone is tall and narrow, so the next one stands beside it, and when
// the row is full the extra ones step slightly off the first column.
func defaultWindow(n, sw, sh int) windowState {
	a, ok := screenWorkArea()
	if !ok {
		return defaultWindowIn(windowRect{X: 80, Y: 80, W: 400, H: 820}, n, sw, sh)
	}
	return defaultWindowIn(a, n, sw, sh)
}

// defaultWindowIn is defaultWindow against a given work area, without asking
// the screen, so it can be tested. sw/sh are the device's screen dimensions
// (falling back to the iPhone 19.5:9 default 780x1688 when unset).
func defaultWindowIn(a windowRect, n, sw, sh int) windowState {
	if sw <= 0 || sh <= 0 {
		sw, sh = screenW, screenH
	}
	h := a.H - 80
	if h > 860 {
		h = 860
	}
	if h < 500 {
		h = 500
	}
	w := h * sw / sh
	if w > 420 {
		w = 420
	}
	// How many phones stand side by side on this screen; a 1440-wide work
	// area holds four.
	perRow := max((a.W-windowEdge-windowGap)/(w+windowGap), 1)
	open := max(n, 0)
	slot, row := open%perRow, open/perRow
	x := a.X + a.W - windowEdge - w - slot*(w+windowGap) - row*windowCascade
	y := a.Y + 40 + row*windowCascade
	// A small screen still has to show the whole window.
	return windowState{X: max(x, a.X+8), Y: min(y, a.Y+a.H-h-8), W: w, H: h}
}

// SetWindowOnTop stores the flag and reopens the window when one is open, so
// the change is visible immediately.
func (v *Driver) SetWindowOnTop(ctx context.Context, d mobile.Device, on bool) error {
	s, ok := v.loadWindow(d)
	if !ok {
		s = defaultWindow(v.openWindows(), d.ScreenW, d.ScreenH)
	}
	s.OnTop = on
	if err := v.saveWindow(d, s); err != nil {
		return err
	}
	p := v.proc(d.ID)
	if p == nil || p.scrcpy == nil {
		return nil
	}
	v.closeWindow(p)
	v.showOn(ctx, d, p)
	return nil
}

// WindowOnTop reports the stored flag; false when the device has no window
// state yet.
func (v *Driver) WindowOnTop(d mobile.Device) bool {
	s, ok := v.loadWindow(d)
	return ok && s.OnTop
}

// SetWindowMuted stores the audio mute flag and reopens the window when one is
// open so scrcpy switches between opus audio and --no-audio immediately.
func (v *Driver) SetWindowMuted(ctx context.Context, d mobile.Device, muted bool) error {
	s, ok := v.loadWindow(d)
	if !ok {
		s = defaultWindow(v.openWindows(), d.ScreenW, d.ScreenH)
	}
	s.Muted = muted
	if err := v.saveWindow(d, s); err != nil {
		return err
	}
	p := v.proc(d.ID)
	if p == nil || p.scrcpy == nil {
		return nil
	}
	v.closeWindow(p)
	v.showOn(ctx, d, p)
	return nil
}

// WindowMuted reports whether the device's scrcpy window audio is muted.
func (v *Driver) WindowMuted(d mobile.Device) bool {
	s, ok := v.loadWindow(d)
	return ok && s.Muted
}

// arrangeLayout computes non-overlapping window positions for count devices in
// work area a. Supported layouts: "row" (side by side), "grid2x2", "grid3x2",
// and "cascade".
func arrangeLayout(a windowRect, layout string, count, sw, sh int) []windowState {
	if count <= 0 {
		return nil
	}
	if sw <= 0 || sh <= 0 {
		sw, sh = screenW, screenH
	}
	out := make([]windowState, count)
	switch layout {
	case "grid2x2", "grid3x2":
		cols, rows := 2, 2
		if layout == "grid3x2" {
			cols, rows = 3, 2
		}
		cellH := max((a.H-windowGap*(rows+1))/rows, 340)
		cellW := min(cellH*sw/sh, max((a.W-windowGap*(cols+1))/cols, 200))
		cellH = min(cellW*sh/sw, cellH)
		startX := a.X + max((a.W-(cols*cellW+(cols-1)*windowGap))/2, windowGap)
		startY := a.Y + max((a.H-(rows*cellH+(rows-1)*windowGap))/2, windowGap)
		for i := 0; i < count; i++ {
			slot := i % (cols * rows)
			layer := i / (cols * rows)
			c, r := slot%cols, slot/cols
			x := startX + c*(cellW+windowGap) + layer*windowCascade
			y := startY + r*(cellH+windowGap) + layer*windowCascade
			out[i] = windowState{
				X: min(max(x, a.X+8), max(a.X+a.W-cellW-8, a.X+8)),
				Y: min(max(y, a.Y+8), max(a.Y+a.H-cellH-8, a.Y+8)),
				W: cellW, H: cellH,
			}
		}
	case "cascade":
		base := defaultWindowIn(a, 0, sw, sh)
		for i := 0; i < count; i++ {
			x := max(base.X-i*windowCascade, a.X+8)
			y := min(base.Y+i*windowCascade, max(a.Y+a.H-base.H-8, a.Y+8))
			out[i] = windowState{X: x, Y: y, W: base.W, H: base.H}
		}
	default: // "row"
		for i := 0; i < count; i++ {
			out[i] = defaultWindowIn(a, i, sw, sh)
		}
	}
	return out
}

// ArrangeWindows repositions every open scrcpy window in devs according to layout
// ("row", "grid2x2", "grid3x2", "cascade") and persists the new positions.
func (v *Driver) ArrangeWindows(ctx context.Context, layout string, devs []mobile.Device) int {
	a, ok := screenWorkArea()
	if !ok {
		a = windowRect{X: 0, Y: 30, W: 1440, H: 840}
	}
	var open []mobile.Device
	for _, d := range devs {
		if p := v.proc(d.ID); p != nil && p.scrcpy != nil {
			open = append(open, d)
		}
	}
	if len(open) == 0 {
		return 0
	}
	slots := arrangeLayout(a, layout, len(open), open[0].ScreenW, open[0].ScreenH)
	for i, d := range open {
		prev, _ := v.loadWindow(d)
		s := slots[i]
		s.OnTop, s.Muted = prev.OnTop, prev.Muted
		_ = v.saveWindow(d, s)
		if p := v.proc(d.ID); p != nil {
			v.closeWindow(p)
			v.showOn(ctx, d, p)
		}
	}
	return len(open)
}
