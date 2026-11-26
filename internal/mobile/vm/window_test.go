// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package vm

import "testing"

// A phone window stands cleanly from left to right in iPhone 19.5:9 proportion,
// and every device opened after it stands beside it instead of overlapping.
func TestDefaultWindowStandsPhonesSideBySide(t *testing.T) {
	area := windowRect{X: 0, Y: 30, W: 1440, H: 812}
	first := defaultWindowIn(area, 0, 0, 0)
	// h = 732; w = 732 * 780 / 1688 = 338 (iPhone 19.5:9), left margin 16, vertically centered.
	if first != (windowState{X: 16, Y: 70, W: 338, H: 732}) {
		t.Fatalf("first = %+v", first)
	}
	// Four 338-point phones and three 10-point gaps fit across the work area left-to-right.
	for n := 0; n < 4; n++ {
		got := defaultWindowIn(area, n, 780, 1688)
		want := first.X + n*(first.W+windowGap)
		if got.X != want || got.Y != first.Y {
			t.Fatalf("phone %d = %+v, want x %d y %d", n, got, want, first.Y)
		}
	}
	// The fifth does not fit in one row: it steps off the first column.
	if got := defaultWindowIn(area, 4, 780, 1688); got.X != first.X+windowCascade || got.Y != first.Y+windowCascade {
		t.Fatalf("fifth = %+v", got)
	}
	// Retina 3x iPhone (1170x2532) keeps the same 19.5:9 window width.
	retina := defaultWindowIn(area, 0, 1170, 2532)
	if retina.W != first.W || retina.H != first.H {
		t.Fatalf("retina = %+v, want %dx%d", retina, first.W, first.H)
	}
}

func TestDefaultWindowStaysInsideTheScreen(t *testing.T) {
	areas := []windowRect{
		{X: 0, Y: 25, W: 1024, H: 700},
		{X: 0, Y: 30, W: 800, H: 600},
		{X: -1200, Y: 30, W: 1200, H: 800}, // a second display to the left
	}
	for _, a := range areas {
		for n := 0; n <= 8; n++ {
			w := defaultWindowIn(a, n, 780, 1688)
			if w.X < a.X || w.Y < a.Y || w.X+w.W > a.X+a.W || w.Y+w.H > a.Y+a.H {
				t.Fatalf("n=%d leaves %+v: %+v", n, a, w)
			}
		}
	}
}

func TestArrangeLayout(t *testing.T) {
	area := windowRect{X: 0, Y: 30, W: 1440, H: 870}
	for _, layout := range []string{"row", "grid2x2", "grid3x2", "cascade"} {
		slots := arrangeLayout(area, layout, 4, 780, 1688)
		if len(slots) != 4 {
			t.Fatalf("%s: got %d slots", layout, len(slots))
		}
		for i, s := range slots {
			if s.W <= 0 || s.H <= 0 || s.X < area.X || s.Y < area.Y {
				t.Fatalf("%s[%d] out of bounds: %+v", layout, i, s)
			}
		}
		if slots[0].X == slots[1].X && slots[0].Y == slots[1].Y {
			t.Fatalf("%s placed windows 0 and 1 on top of each other", layout)
		}
	}
}
