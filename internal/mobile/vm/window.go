// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package vm

// windowState is where a device window was last seen, in screen points from
// the top left, plus its always-on-top and audio mute flags.
type windowState struct {
	X         int  `json:"x"`
	Y         int  `json:"y"`
	W         int  `json:"w"`
	H         int  `json:"h"`
	OnTop     bool `json:"onTop"`
	Muted     bool `json:"muted,omitempty"`
	Minimized bool `json:"minimized,omitempty"`
}

const (
	screenW = 780
	screenH = 1688

	// windowEdge is the gap kept from the right edge of the screen.
	windowEdge = 28
	// windowGap is the strip between two phones standing side by side.
	windowGap = 8
	// windowCascade is how far a phone that does not fit across the screen
	// steps from the one below it, so it is still visibly a separate window.
	windowCascade = 24
)

// defaultWindow puts a phone-shaped window at the right edge of the screen,
// clear of the Dock and the menu bar.
func defaultWindow(n, sw, sh int) windowState {
	a, ok := screenWorkArea()
	if !ok {
		return defaultWindowIn(windowRect{X: 80, Y: 80, W: 400, H: 820}, n, sw, sh)
	}
	return defaultWindowIn(a, n, sw, sh)
}

// defaultWindowIn is defaultWindow against a given work area, without asking
// the screen, so it can be tested.
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
	perRow := max((a.W-windowEdge-windowGap)/(w+windowGap), 1)
	open := max(n, 0)
	slot, row := open%perRow, open/perRow
	x := a.X + a.W - windowEdge - w - slot*(w+windowGap) - row*windowCascade
	y := a.Y + 40 + row*windowCascade
	return windowState{X: max(x, a.X+8), Y: min(y, a.Y+a.H-h-8), W: w, H: h}
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
