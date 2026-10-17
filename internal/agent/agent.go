// SPDX-License-Identifier: AGPL-3.0-or-later

// Package agent drives a profile's browser for an AI agent, over the DevTools
// pipe the app already holds: no DevTools port is opened. It reads pages in an
// isolated world and never calls Runtime.enable, and it clicks and types with
// real input events, so the page sees a person's input, not a script's.
package agent

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/cdppipe"
)

// Conn is the DevTools pipe of one running profile (a *cdppipe.Conn).
type Conn interface {
	Call(ctx context.Context, session, method string, params any) (json.RawMessage, error)
	Subscribe(size int) (<-chan cdppipe.Event, func())
	Done() <-chan struct{}
}

// Hub keeps what the agent knows about each profile's browser between calls.
type Hub struct {
	// OnActive is told when a profile starts or stops being driven. It runs
	// without the hub's lock held.
	OnActive func(profileID string, active bool)
	idle     time.Duration

	mu       sync.Mutex
	browsers map[string]*Browser
	quit     chan struct{}
	stopped  sync.Once
}

// NewHub returns a hub that counts a profile as driven until the agent has
// been idle for idle.
func NewHub(idle time.Duration) *Hub {
	h := &Hub{idle: idle, browsers: map[string]*Browser{}, quit: make(chan struct{})}
	go h.reap()
	return h
}

// Close stops the idle timer.
func (h *Hub) Close() { h.stopped.Do(func() { close(h.quit) }) }

// Browser is the agent's view of one profile's browser. Its methods are the
// tools; each one counts as activity.
type Browser struct {
	hub       *Hub
	profileID string
	conn      Conn

	mu      sync.Mutex
	tabs    map[string]*tab // by target id
	current string          // target id the agent used last
	lastUse time.Time
	active  bool
}

type tab struct {
	targetID string
	session  string
	net      *netLog
	// The isolated world the agent reads the page in, for one document.
	worldLoader string
	worldID     int
	mouseX      float64
	mouseY      float64
}

// For returns the browser of a running profile, starting over when the
// profile was restarted (a new pipe).
func (h *Hub) For(profileID string, c Conn) *Browser {
	h.mu.Lock()
	b := h.browsers[profileID]
	if b != nil && b.conn == c {
		h.mu.Unlock()
		return b
	}
	b = &Browser{hub: h, profileID: profileID, conn: c, tabs: map[string]*tab{}}
	h.browsers[profileID] = b
	h.mu.Unlock()
	events, cancel := c.Subscribe(1024)
	go b.listen(events, cancel)
	return b
}

// Active lists the profiles being driven now, sorted.
func (h *Hub) Active() []string {
	h.mu.Lock()
	bs := make([]*Browser, 0, len(h.browsers))
	for _, b := range h.browsers {
		bs = append(bs, b)
	}
	h.mu.Unlock()
	out := []string{}
	for _, b := range bs {
		b.mu.Lock()
		if b.active {
			out = append(out, b.profileID)
		}
		b.mu.Unlock()
	}
	sort.Strings(out)
	return out
}

func (h *Hub) reap() {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-h.quit:
			return
		case <-t.C:
			h.mu.Lock()
			bs := make([]*Browser, 0, len(h.browsers))
			for _, b := range h.browsers {
				bs = append(bs, b)
			}
			h.mu.Unlock()
			for _, b := range bs {
				b.mu.Lock()
				idle := b.active && time.Since(b.lastUse) > h.idle
				if idle {
					b.active = false
				}
				b.mu.Unlock()
				if idle {
					h.report(b.profileID, false)
				}
			}
		}
	}
}

func (h *Hub) report(profileID string, active bool) {
	if h.OnActive != nil {
		h.OnActive(profileID, active)
	}
}

// touch marks the browser as being driven now.
func (b *Browser) touch() {
	b.mu.Lock()
	was := b.active
	b.active, b.lastUse = true, time.Now()
	b.mu.Unlock()
	if !was {
		b.hub.report(b.profileID, true)
	}
}

// listen files network events under their tab and forgets closed tabs, until
// the pipe closes; then the hub forgets the browser.
func (b *Browser) listen(events <-chan cdppipe.Event, cancel func()) {
	defer cancel()
	for ev := range events {
		switch {
		case ev.Method == "Target.detachedFromTarget":
			var p struct {
				SessionID string `json:"sessionId"`
			}
			json.Unmarshal(ev.Params, &p)
			b.mu.Lock()
			for id, t := range b.tabs {
				if t.session == p.SessionID {
					delete(b.tabs, id)
				}
			}
			b.mu.Unlock()
		case len(ev.Method) > 8 && ev.Method[:8] == "Network.":
			b.mu.Lock()
			for _, t := range b.tabs {
				if t.session == ev.SessionID {
					t.net.add(ev.Method, ev.Params)
					break
				}
			}
			b.mu.Unlock()
		}
	}
	h := b.hub
	h.mu.Lock()
	if h.browsers[b.profileID] == b {
		delete(h.browsers, b.profileID)
	}
	h.mu.Unlock()
	b.mu.Lock()
	was := b.active
	b.active = false
	b.mu.Unlock()
	if was {
		h.report(b.profileID, false)
	}
}

func (b *Browser) call(ctx context.Context, session, method string, params any, out any) error {
	raw, err := b.conn.Call(ctx, session, method, params)
	if err != nil {
		return err
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func errGone(ref string) error {
	return apperr.Newf("agent_ref_gone", "%s is no longer on the page; take a new snapshot", ref)
}
