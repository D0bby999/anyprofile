// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import "testing"

func TestFirstWindow(t *testing.T) {
	// A 13-inch MacBook: 1440x900 points, 30 under the menu bar, 812 of work
	// area left. The window keeps 22 points around it vertically and 44 in
	// total horizontally, the way Chromium's own default does.
	mb := Rect{0, 30, 1440, 812}
	if got, want := firstWindow(mb, 1), (Rect{X: 80, Y: 52, W: 1280, H: 768}); got != want {
		t.Fatalf("macbook = %+v, want %+v", got, want)
	}
	// A big screen gets the same window, centred: not stretched across it.
	if got, want := firstWindow(Rect{0, 0, 2560, 1360}, 1), (Rect{X: 640, Y: 230, W: 1280, H: 900}); got != want {
		t.Fatalf("wide = %+v, want %+v", got, want)
	}
	// A small screen: the floor yields to the screen, never the other way.
	if got, want := firstWindow(Rect{0, 25, 1024, 720}, 1), (Rect{X: 22, Y: 47, W: 980, H: 676}); got != want {
		t.Fatalf("small = %+v, want %+v", got, want)
	}
	// Every other window still opening steps down and right, so a second
	// first-open window is visibly not the first one.
	if got := firstWindow(mb, 2); got.X != 104 || got.Y != 74 { // Y is pulled back inside
		t.Fatalf("second = %+v", got)
	}
	if got := firstWindow(mb, 4); got.X != 152 || got.Y != 74 {
		t.Fatalf("fourth = %+v", got)
	}
	// The step wraps after four, so the fifth window lands like the first.
	if got := firstWindow(mb, 5); got != firstWindow(mb, 1) {
		t.Fatalf("fifth = %+v", got)
	}
	// The step is pulled back inside the area instead of leaving the screen.
	area := Rect{0, 30, 1200, 700}
	for n := 1; n <= 9; n++ {
		r := firstWindow(area, n)
		if r.X < area.X || r.Y < area.Y || r.X+r.W > area.X+area.W || r.Y+r.H > area.Y+area.H {
			t.Fatalf("n=%d leaves the area: %+v in %+v", n, r, area)
		}
	}
}
