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

	// windowEdge is the outer margin kept from the visible work-area edges.
	windowEdge = 16
	// windowGap is the uniform spacing between phones in rows and grids.
	windowGap = 10
	// windowCascade is how far cascaded or wrapped windows step apart.
	windowCascade = 28
)

// defaultWindow puts a phone-shaped window in slot n from left to right inside
// the visible work area (clear of the Dock and menu bar), MuMu Player style.
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
	if h > 840 {
		h = 840
	}
	if h < 460 {
		h = 460
	}
	if h > a.H-2*windowEdge {
		h = max(a.H-2*windowEdge, 300)
	}
	w := h * sw / sh
	if w > 420 {
		w = 420
		h = w * sh / sw
	}
	perRow := max((a.W-2*windowEdge+windowGap)/(w+windowGap), 1)
	open := max(n, 0)
	slot, row := open%perRow, open/perRow
	baseY := a.Y + max((a.H-h)/2, windowEdge)
	x := a.X + windowEdge + slot*(w+windowGap) + row*windowCascade
	y := baseY + row*windowCascade
	maxX := max(a.X+a.W-w-8, a.X+8)
	maxY := max(a.Y+a.H-h-8, a.Y+8)
	return windowState{X: min(max(x, a.X+8), maxX), Y: min(max(y, a.Y+8), maxY), W: w, H: h}
}

func gridSlots(a windowRect, cols, rows, count, sw, sh int) []windowState {
	availW := max(a.W-2*windowEdge-(cols-1)*windowGap, 200)
	availH := max(a.H-2*windowEdge-(rows-1)*windowGap, 260)
	maxCellW := max(availW/cols, 180)
	maxCellH := max(availH/rows, 260)
	cellH := min(maxCellH, maxCellW*sh/sw)
	if cellH > 860 {
		cellH = 860
	}
	cellW := cellH * sw / sh
	if cellW > maxCellW {
		cellW = maxCellW
		cellH = cellW * sh / sw
	}
	totalW := cols*cellW + (cols-1)*windowGap
	totalH := rows*cellH + (rows-1)*windowGap
	startX := a.X + max((a.W-totalW)/2, windowEdge)
	startY := a.Y + max((a.H-totalH)/2, windowEdge)
	out := make([]windowState, count)
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
	return out
}

// arrangeLayout computes clean, non-overlapping MuMu-style window positions for
// count devices in work area a. Supported layouts: "row", "grid2x2", "grid3x2",
// and "cascade".
func arrangeLayout(a windowRect, layout string, count, sw, sh int) []windowState {
	if count <= 0 {
		return nil
	}
	if sw <= 0 || sh <= 0 {
		sw, sh = screenW, screenH
	}
	switch layout {
	case "grid2x2":
		rows := 1
		if count > 2 {
			rows = 2
		}
		if count > 8 {
			rows = 3
		}
		cols := (count + rows - 1) / rows
		return gridSlots(a, cols, rows, count, sw, sh)
	case "grid3x2":
		rows := 1
		if count > 3 {
			rows = 2
		}
		if count > 6 {
			rows = 3
		}
		cols := (count + rows - 1) / rows
		return gridSlots(a, cols, rows, count, sw, sh)
	case "cascade":
		base := defaultWindowIn(a, 0, sw, sh)
		out := make([]windowState, count)
		for i := 0; i < count; i++ {
			x := min(a.X+windowEdge+i*windowCascade, max(a.X+a.W-base.W-8, a.X+8))
			y := min(a.Y+windowEdge+i*windowCascade, max(a.Y+a.H-base.H-8, a.Y+8))
			out[i] = windowState{X: x, Y: y, W: base.W, H: base.H}
		}
		return out
	default: // "row": MuMu side-by-side left-to-right, wrapping to a clean grid if >1 row needed
		base := defaultWindowIn(a, 0, sw, sh)
		perRow := max((a.W-2*windowEdge+windowGap)/(base.W+windowGap), 1)
		if count <= perRow {
			out := make([]windowState, count)
			for i := 0; i < count; i++ {
				out[i] = defaultWindowIn(a, i, sw, sh)
			}
			return out
		}
		// Pick the row count (2..3) that keeps every phone readable without overlap.
		rows := 2
		if count > perRow*2 {
			rows = 3
		}
		cols := (count + rows - 1) / rows
		return gridSlots(a, cols, rows, count, sw, sh)
	}
}
