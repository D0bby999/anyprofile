// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/D0bby999/anyprofile/internal/engine/apostate"
	"github.com/D0bby999/anyprofile/internal/store"
)

// Profiles opened together, in whatever order they finish starting, get
// their windows laid out in the order the list asks for, without coming to
// the front.
func TestArrangeFollowsTheListOrder(t *testing.T) {
	r := newApostateRig(t)
	l := r.launcher(t)
	l.Capture = true // the pipe, as in the app
	ids := make([]string, 4)
	for i := range ids {
		ids[i] = fmt.Sprintf("arr%d", i)
		if err := r.st.CreateProfile(&store.Profile{ID: ids[i], Name: ids[i], Engine: "apostate",
			EngineVersion: apostate.Version, PersonaID: apostate.HostPersona, Platform: apostate.HostPlatform,
			SeedHex: fmt.Sprintf("00000000000a11c%d", i), Timezone: "UTC", Languages: "en-US,en", UILocale: "en-US",
			EngineMeta: store.EngineMeta{HWC: 4, MemGiB: 8}}); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if err := l.StartBehind(context.Background(), id); err != nil {
				t.Errorf("start %s: %v", id, err)
			}
		}(id)
	}
	wg.Wait()
	defer func() {
		for _, id := range ids {
			l.Stop(id)
		}
	}()
	area := Rect{0, 25, 1600, 975}
	screenArea = func() (Rect, bool) { return area, true }
	defer func() { screenArea = workArea }()
	order := []string{ids[2], ids[0], ids[3], ids[1]}
	n, err := l.Arrange(context.Background(), order, false)
	if err != nil || n != 4 {
		t.Fatalf("arranged %d: %v", n, err)
	}
	cells := grid(area, 4)
	for i, id := range order {
		c, err := l.WaitConn(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		ts, err := pageTargets(context.Background(), c)
		if err != nil || len(ts) == 0 {
			t.Fatalf("pages of %s: %v", id, err)
		}
		raw, err := c.Call(context.Background(), "", "Browser.getWindowForTarget", map[string]any{"targetId": ts[0].TargetID})
		if err != nil {
			t.Fatal(err)
		}
		var w struct {
			Bounds struct{ Left, Top, Width, Height int } `json:"bounds"`
		}
		json.Unmarshal(raw, &w)
		if w.Bounds.Left != cells[i].X || w.Bounds.Top != cells[i].Y {
			t.Errorf("window %d (%s) at %d,%d; want cell %d at %d,%d", i, id, w.Bounds.Left, w.Bounds.Top, i, cells[i].X, cells[i].Y)
		}
	}
}
