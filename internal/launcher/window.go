// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

// What a profile's very first window looks like. Chromium remembers the size
// and position of every later window by itself (browser.window_placement in
// the profile), so this decides only the one the user has not made yet.
const (
	// Chrome's own macOS default is min(work width - 44, 1200) by
	// (work height - 44): 22 points around the window, capped at 1200 so the
	// page stays the size most sites are laid out for. 1280 is the same idea
	// with room for a work window: it fits a 13-inch MacBook's 1440-wide
	// screen with the desktop still visible around it, and stays above the
	// 1008-pixel breakpoint where a page switches to its wide layout.
	firstW, firstH = 1280, 900
	// A larger screen does not get a larger window: past this the page is
	// mostly empty margins, and a window taller than 900 is hard to read.
	maxFirstW, maxFirstH = 1280, 900
	// Below this a page is cramped. A screen smaller than the floor still
	// wins: the window never leaves the work area.
	minFirstW, minFirstH = 960, 600
	// windowMargin is what Chromium leaves around a new window on macOS
	// (22 points each side).
	windowMargin = 44
	// cascade keeps a second, a third window visibly offset from the one
	// before, the way macOS and every browser open their own windows.
	cascade    = 24
	cascadeMax = 4
)

// firstWindow is the bounds of a profile's first window, in points from the
// top left of the main screen, as Chromium's own window bounds are (the outer
// frame, not the page area). n is how many browsers are already opening or
// running: each of them shifts the new window one step, so two windows never
// land exactly on top of each other.
func firstWindow(area Rect, n int) Rect {
	// What the work area can hold with a margin, then what the screen type is
	// worth opening at.
	room := Rect{X: area.X, Y: area.Y, W: max(area.W-windowMargin, 1), H: max(area.H-windowMargin, 1)}
	w := min(min(maxFirstW, room.W), max(minFirstW, room.W))
	h := min(min(maxFirstH, room.H), max(minFirstH, room.H))
	// The floor yields to a screen that cannot hold it.
	w, h = min(w, area.W), min(h, area.H)
	// Centred, then stepped down and right for every other browser, and
	// pulled back inside the area when the step would leave it.
	step := cascade * (max(n-1, 0) % cascadeMax)
	x := min(area.X+(area.W-w)/2+step, area.X+area.W-w)
	y := min(area.Y+(area.H-h)/2+step, area.Y+area.H-h)
	return Rect{X: x, Y: y, W: w, H: h}
}
