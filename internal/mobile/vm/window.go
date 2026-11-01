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
// points from the top left, plus its always-on-top flag.
type windowState struct {
	X     int  `json:"x"`
	Y     int  `json:"y"`
	W     int  `json:"w"`
	H     int  `json:"h"`
	OnTop bool `json:"onTop"`
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
func defaultWindow(n int) windowState {
	a, ok := screenWorkArea()
	if !ok {
		return defaultWindowIn(windowRect{X: 80, Y: 80, W: 400, H: 820}, n)
	}
	return defaultWindowIn(a, n)
}

// defaultWindowIn is defaultWindow against a given work area, without asking
// the screen, so it can be tested.
func defaultWindowIn(a windowRect, n int) windowState {
	h := a.H - 80
	if h > 860 {
		h = 860
	}
	if h < 500 {
		h = 500
	}
	w := h * 9 / 20 // 720x1600 is 9:20
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
		s = defaultWindow(v.openWindows())
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
