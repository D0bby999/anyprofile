// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestGateUnlimitedNeverBlocks(t *testing.T) {
	g := newLaunchGate()
	ctx := context.Background()
	for i := 0; i < 50; i++ {
		if err := g.Acquire(ctx); err != nil {
			t.Fatalf("acquire %d: %v", i, err)
		}
	}
	if got := g.Running(); got != 50 {
		t.Fatalf("running = %d, want 50", got)
	}
	if got := g.Waiting(); got != 0 {
		t.Fatalf("waiting = %d, want 0", got)
	}
}

func TestGateBlocksAtCeilingUntilRelease(t *testing.T) {
	g := newLaunchGate()
	g.SetMax(2)
	ctx := context.Background()
	if err := g.Acquire(ctx); err != nil {
		t.Fatal(err)
	}
	if err := g.Acquire(ctx); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- g.Acquire(ctx) }()
	select {
	case err := <-done:
		t.Fatalf("third acquire returned %v before a release", err)
	case <-time.After(100 * time.Millisecond):
	}
	if got := g.Waiting(); got != 1 {
		t.Fatalf("waiting = %d, want 1", got)
	}
	g.Release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("third acquire after release: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("third acquire still blocked after a release")
	}
	if got := g.Running(); got != 2 {
		t.Fatalf("running = %d, want 2", got)
	}
}

func TestGateAcquireHonoursContextCancel(t *testing.T) {
	g := newLaunchGate()
	g.SetMax(1)
	if err := g.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- g.Acquire(ctx) }()
	// Let the waiter register before cancelling.
	deadline := time.Now().Add(2 * time.Second)
	for g.Waiting() != 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled acquire returned nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled acquire did not return")
	}
	if got := g.Running(); got != 1 {
		t.Fatalf("running = %d, want 1 (cancel must not take a slot)", got)
	}
	if got := g.Waiting(); got != 0 {
		t.Fatalf("waiting = %d, want 0", got)
	}
}

func TestGateSetMaxRaiseWakesWaiters(t *testing.T) {
	g := newLaunchGate()
	g.SetMax(1)
	ctx := context.Background()
	if err := g.Acquire(ctx); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = g.Acquire(ctx)
		}()
	}
	deadline := time.Now().Add(2 * time.Second)
	for g.Waiting() != 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	g.SetMax(3)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("waiter %d: %v", i, err)
		}
	}
	if got := g.Running(); got != 3 {
		t.Fatalf("running = %d, want 3", got)
	}
}

func TestGateReleaseNeverNegative(t *testing.T) {
	g := newLaunchGate()
	g.Release()
	g.Release()
	if got := g.Running(); got != 0 {
		t.Fatalf("running = %d, want 0", got)
	}
}
