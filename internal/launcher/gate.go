// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"context"
	"sync"
)

// launchGate counts all starting/running browsers, including adopted ones.
// Zero means unlimited. Waiters are admitted FIFO; cancellation removes the
// waiter without taking a slot. Every admitted/adopted browser releases once.
type gateWaiter struct{ token byte }

type launchGate struct {
	mu           sync.Mutex
	max, running int
	queue        []*gateWaiter
	changed      chan struct{}
}

func newLaunchGate() *launchGate { return &launchGate{changed: make(chan struct{})} }

func (g *launchGate) broadcastLocked() {
	close(g.changed)
	g.changed = make(chan struct{})
}

func (g *launchGate) SetMax(n int) {
	g.mu.Lock()
	g.max = max(n, 0)
	g.broadcastLocked()
	g.mu.Unlock()
}

func (g *launchGate) Max() int     { g.mu.Lock(); defer g.mu.Unlock(); return g.max }
func (g *launchGate) Running() int { g.mu.Lock(); defer g.mu.Unlock(); return g.running }
func (g *launchGate) Waiting() int { g.mu.Lock(); defer g.mu.Unlock(); return len(g.queue) }

// Adopt accounts for an already running browser even when it exceeds the
// ceiling. New launches then wait until enough existing browsers have exited.
func (g *launchGate) Adopt() {
	g.mu.Lock()
	g.running++
	g.mu.Unlock()
}

func (g *launchGate) Acquire(ctx context.Context) error {
	// Kept separate from the wait loop so admission always checks cancellation,
	// including when the gate is unlimited or a release races cancellation.
	g.mu.Lock()
	defer g.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	// A unique queue entry identifies this waiter independently of its context.
	entry := &gateWaiter{}
	g.queue = append(g.queue, entry)
	remove := func() {
		for i, other := range g.queue {
			if other == entry {
				g.queue = append(g.queue[:i], g.queue[i+1:]...)
				break
			}
		}
		g.broadcastLocked()
	}
	for {
		if err := ctx.Err(); err != nil {
			remove()
			return err
		}
		if g.queue[0] == entry && (g.max == 0 || g.running < g.max) {
			g.running++
			remove()
			return nil
		}
		ch := g.changed
		g.mu.Unlock()
		select {
		case <-ctx.Done():
		case <-ch:
		}
		g.mu.Lock()
	}
}

func (g *launchGate) Release() {
	g.mu.Lock()
	if g.running > 0 {
		g.running--
	}
	g.broadcastLocked()
	g.mu.Unlock()
}
