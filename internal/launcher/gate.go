// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"context"
	"sync"
)

// launchGate bounds how many browsers may be starting or running at once.
// A max of 0 means unlimited, which is the historical behaviour: the gate
// is only a ceiling once the app sets one from its settings. Starts beyond
// the ceiling wait (in arrival order is not guaranteed) until a running
// browser exits or the ceiling is raised; a cancelled context abandons the
// wait. The gate counts launches admitted by open(); browsers adopted from
// a previous app run are not gated and never release a slot.
type launchGate struct {
	mu      sync.Mutex
	max     int
	running int
	waiting int
	changed chan struct{} // closed and replaced on every state change
}

func newLaunchGate() *launchGate {
	return &launchGate{changed: make(chan struct{})}
}

// broadcastLocked wakes every waiter so it can re-check the counts.
// Callers hold mu.
func (g *launchGate) broadcastLocked() {
	close(g.changed)
	g.changed = make(chan struct{})
}

// SetMax changes the ceiling; 0 (or negative) means unlimited. Raising the
// ceiling wakes waiters so they can take the new slots.
func (g *launchGate) SetMax(n int) {
	if n < 0 {
		n = 0
	}
	g.mu.Lock()
	g.max = n
	g.broadcastLocked()
	g.mu.Unlock()
}

// Max reports the current ceiling, 0 for unlimited.
func (g *launchGate) Max() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.max
}

// Running reports how many admitted launches have not released yet.
func (g *launchGate) Running() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.running
}

// Waiting reports how many Acquire calls are currently blocked.
func (g *launchGate) Waiting() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.waiting
}

// Acquire takes one slot, blocking while the ceiling is reached. It
// returns the context's error when the wait is abandoned; no slot is
// taken in that case. Every successful Acquire must be paired with
// exactly one Release.
func (g *launchGate) Acquire(ctx context.Context) error {
	g.mu.Lock()
	for g.max > 0 && g.running >= g.max {
		g.waiting++
		ch := g.changed
		g.mu.Unlock()
		select {
		case <-ctx.Done():
			g.mu.Lock()
			g.waiting--
			g.mu.Unlock()
			return ctx.Err()
		case <-ch:
		}
		g.mu.Lock()
		g.waiting--
	}
	g.running++
	g.mu.Unlock()
	return nil
}

// Release returns one slot and wakes the waiters. Releasing more often
// than Acquire succeeded is a programming error; the count never drops
// below zero so one bad call cannot corrupt the gate.
func (g *launchGate) Release() {
	g.mu.Lock()
	if g.running > 0 {
		g.running--
	}
	g.broadcastLocked()
	g.mu.Unlock()
}
