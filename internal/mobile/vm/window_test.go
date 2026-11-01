// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package vm

import "testing"

// A phone window stands at the right edge, and every device opened after it
// stands beside it instead of landing on exactly the same spot.
func TestDefaultWindowStandsPhonesSideBySide(t *testing.T) {
	area := windowRect{X: 0, Y: 30, W: 1440, H: 812}
	first := defaultWindowIn(area, 0)
	if first != (windowState{X: 1083, Y: 70, W: 329, H: 732}) {
		t.Fatalf("first = %+v", first)
	}
	// Four 329-point phones and three 8-point gaps fit across the work area.
	for n := 0; n < 4; n++ {
		got := defaultWindowIn(area, n)
		want := first.X - n*(first.W+windowGap)
		if got.X != want || got.Y != first.Y {
			t.Fatalf("phone %d = %+v, want x %d y %d", n, got, want, first.Y)
		}
	}
	if last := defaultWindowIn(area, 3); last.X != 72 {
		t.Fatalf("fourth phone at %d, want 72", last.X)
	}
	// The fifth does not fit: it steps off the first column so it is still a
	// separate window rather than a copy of the first.
	if got := defaultWindowIn(area, 4); got.X != first.X-windowCascade || got.Y != first.Y+windowCascade {
		t.Fatalf("fifth = %+v", got)
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
			w := defaultWindowIn(a, n)
			if w.X < a.X || w.Y < a.Y || w.X+w.W > a.X+a.W || w.Y+w.H > a.Y+a.H {
				t.Fatalf("n=%d leaves %+v: %+v", n, a, w)
			}
		}
	}
}
