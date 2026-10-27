// SPDX-License-Identifier: AGPL-3.0-or-later
//go:build integration

package launcher

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/engine/apostate"
	"github.com/D0bby999/anyprofile/internal/store"
)

func TestApostateAdoptedBrowserCountsTowardCeiling(t *testing.T) {
	r := newApostateRig(t)
	original := r.launcher(t)
	original.Capture = true
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	defer original.StopAll()
	for _, id := range []string{"adopted", "pending"} {
		if err := r.st.CreateProfile(&store.Profile{ID: id, Name: id, Engine: "apostate", EngineVersion: apostate.Version,
			PersonaID: apostate.HostPersona, Platform: apostate.HostPlatform, SeedHex: "00000000000a11ce", Timezone: "UTC", Languages: "en-US,en", UILocale: "en-US", EngineMeta: store.EngineMeta{HWC: 4, MemGiB: 8}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := original.StartBehind(ctx, "adopted"); err != nil {
		t.Fatal(err)
	}
	adopted := r.launcher(t)
	adopted.SetMaxRunning(1)
	adopted.Adopt()
	defer adopted.StopAll()
	if got := adopted.gate.Running(); got != 1 {
		t.Fatalf("adopted count = %d", got)
	}
	done := make(chan error, 1)
	go func() { done <- adopted.StartBehind(ctx, "pending") }()
	waitForQueue(t, adopted.gate, 1)
	if in, _ := adopted.Info("pending"); in.State != Queued {
		t.Fatalf("pending state = %+v", in)
	}
	if err := adopted.Stop("pending"); err != nil {
		t.Fatal(err)
	}
	<-done
	adopted.StopAll()
	if adopted.gate.Running() != 0 {
		t.Fatal("adopted slot was not released")
	}
}

// Uses throwaway profile directories, the verified installed engine and a mock
// keychain. No user profiles, real proxies or credentials are touched.
func TestApostateLaunchCeilingQueue(t *testing.T) {
	r := newApostateRig(t)
	l := r.launcher(t)
	l.Capture = true
	l.SetMaxRunning(2)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	defer l.StopAll()
	for i := 0; i < 4; i++ {
		id := fmt.Sprintf("cap%d", i)
		if err := r.st.CreateProfile(&store.Profile{ID: id, Name: id, Engine: "apostate", EngineVersion: apostate.Version, PersonaID: apostate.HostPersona, Platform: apostate.HostPlatform, SeedHex: fmt.Sprintf("%016x", i+123), Timezone: "UTC", Languages: "en-US,en", UILocale: "en-US", EngineMeta: store.EngineMeta{HWC: 4, MemGiB: 8}}); err != nil {
			t.Fatal(err)
		}
	}
	done := make(chan error, 4)
	for i := 0; i < 4; i++ {
		go func(i int) { done <- l.StartBehind(ctx, fmt.Sprintf("cap%d", i)) }(i)
	}
	waitState := func(running, queued int) []Info {
		t.Helper()
		for ctx.Err() == nil {
			infos := l.List()
			nr, nq := 0, 0
			for _, in := range infos {
				if in.State == Running {
					nr++
				}
				if in.State == Queued {
					nq++
				}
			}
			if nr > 2 {
				t.Fatalf("ceiling exceeded: %+v", infos)
			}
			if nr == running && nq == queued {
				return infos
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("state timeout: %+v", l.List())
		return nil
	}
	infos := waitState(2, 2)
	// Cancel a queued profile: it must never launch after a running slot opens.
	queuedID, stoppedID := "", ""
	for _, in := range infos {
		if in.State == Queued {
			queuedID = in.ProfileID
		}
		if in.State == Running {
			stoppedID = in.ProfileID
		}
	}
	if err := l.Stop(queuedID); err != nil {
		t.Fatal(err)
	}
	if err := l.Stop(stoppedID); err != nil {
		t.Fatal(err)
	}
	waitState(2, 0)
	if _, ok := l.Info(queuedID); ok {
		t.Fatal("canceled queue entry came back")
	}
	// Stop every remaining browser and verify the entire process trees disappear.
	l.StopAll()
	for range 4 {
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("launch call remained blocked")
		}
	}
	for i := 0; i < 4; i++ {
		udd := r.dirs.Profile(fmt.Sprintf("cap%d", i))
		deadline := time.Now().Add(2 * time.Second)
		var left string
		for {
			left = processesUsing(t, udd)
			if left == "" || time.Now().After(deadline) {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if left != "" {
			t.Fatalf("processes left: %s", left)
		}
	}
	if l.gate.Running() != 0 || l.WaitingLaunches() != 0 {
		t.Fatal("gate leaked a slot or waiter")
	}
}
