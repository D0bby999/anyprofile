// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fakeTarget struct {
	id   string
	w, h int
	mu   sync.Mutex
	taps [][2]int
	keys []string
}

func (f *fakeTarget) ID() string              { return f.id }
func (f *fakeTarget) ScreenSize() (int, int)  { return f.w, f.h }
func (f *fakeTarget) Tap(_ context.Context, x, y int) error {
	f.mu.Lock()
	f.taps = append(f.taps, [2]int{x, y})
	f.mu.Unlock()
	return nil
}
func (f *fakeTarget) Swipe(_ context.Context, _, _, _, _, _ int) error { return nil }
func (f *fakeTarget) PressKey(_ context.Context, key string) error {
	f.mu.Lock()
	f.keys = append(f.keys, key)
	f.mu.Unlock()
	return nil
}

func TestHubSynchronizesNormalizedCoordsAndJitter(t *testing.T) {
	hub := NewHub()
	f1 := &fakeTarget{id: "f1", w: 780, h: 1688}
	f2 := &fakeTarget{id: "f2", w: 540, h: 1170}
	hub.Start("master", []Target{f1, f2}, Jitter{Enabled: true, DelayMinMS: 5, DelayMaxMS: 15, OffsetPX: 4})

	st := hub.Status()
	if !st.Active || st.MasterID != "master" || len(st.Followers) != 2 {
		t.Fatalf("status = %+v", st)
	}
	ctx := context.Background()
	hub.DispatchTap(ctx, "master", 0.5, 0.5)
	hub.DispatchKey(ctx, "master", "home")
	time.Sleep(60 * time.Millisecond)

	f1.mu.Lock()
	defer f1.mu.Unlock()
	f2.mu.Lock()
	defer f2.mu.Unlock()
	if len(f1.taps) != 1 || len(f2.taps) != 1 {
		t.Fatalf("taps f1=%v f2=%v", f1.taps, f2.taps)
	}
	if f1.taps[0][0] < 386 || f1.taps[0][0] > 394 || f2.taps[0][0] < 266 || f2.taps[0][0] > 274 {
		t.Fatalf("scaled taps out of bounds: f1=%v f2=%v", f1.taps[0], f2.taps[0])
	}
	if len(f1.keys) != 1 || f1.keys[0] != "home" {
		t.Fatalf("keys = %v", f1.keys)
	}
	hub.Stop()
	if hub.Status().Active {
		t.Fatal("hub still active after Stop")
	}
}
