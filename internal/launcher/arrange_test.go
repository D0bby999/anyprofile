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
	if got := grid(area, 1); !reflect.DeepEqual(got, []Rect{area}) {
		t.Fatalf("1 = %v", got)
	}
	// Two side by side.
	if got := grid(area, 2); !reflect.DeepEqual(got, []Rect{{0, 25, 600, 800}, {600, 25, 600, 800}}) {
		t.Fatalf("2 = %v", got)
	}
	// Five: three columns, two rows, inside the area.
	got := grid(area, 5)
	want := []Rect{{0, 25, 400, 400}, {400, 25, 400, 400}, {800, 25, 400, 400}, {0, 425, 400, 400}, {400, 425, 400, 400}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("5 = %v", got)
	}
}
