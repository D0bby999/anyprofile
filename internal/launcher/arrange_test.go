// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"reflect"
	"testing"
)

func TestGrid(t *testing.T) {
	area := Rect{X: 0, Y: 25, W: 1200, H: 800}
	if got := grid(area, 0); got != nil {
		t.Fatalf("0 = %v", got)
	}
	// One window keeps the whole area, gap and all.
	if got := grid(area, 1); !reflect.DeepEqual(got, []Rect{area}) {
		t.Fatalf("1 = %v", got)
	}
	// Two side by side, 8 points apart.
	if got := grid(area, 2); !reflect.DeepEqual(got, []Rect{{0, 25, 596, 800}, {604, 25, 596, 800}}) {
		t.Fatalf("2 = %v", got)
	}
	// Five: three columns, two rows, 8 points between them, inside the area.
	got := grid(area, 5)
	want := []Rect{
		{0, 25, 394, 396}, {402, 25, 394, 396}, {804, 25, 394, 396},
		{0, 429, 394, 396}, {402, 429, 394, 396},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("5 = %v", got)
	}
	// Nine fills three by three, and the last row ends at the area's edge.
	nine := grid(area, 9)
	last := nine[len(nine)-1]
	if last.X+last.W > area.X+area.W || last.Y+last.H > area.Y+area.H {
		t.Fatalf("9 leaves the area: %v", last)
	}
}

func TestGridStopsWhenPanesGetTooSmall(t *testing.T) {
	area := Rect{X: 0, Y: 30, W: 1440, H: 812}
	usable := func(n int) bool {
		c := grid(area, n)[0]
		return c.W >= minTileW && c.H >= minTileH
	}
	// Six fit side by side in three columns; a seventh adds a row of slivers.
	for n := 2; n <= 6; n++ {
		if !usable(n) {
			t.Fatalf("%d panes are too small to open tiled", n)
		}
	}
	for n := 7; n <= 9; n++ {
		if usable(n) {
			t.Fatalf("%d panes are small enough to open tiled", n)
		}
	}
}
