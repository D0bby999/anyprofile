// SPDX-License-Identifier: AGPL-3.0-or-later

// Package sync implements a MuMu-style multi-instance Synchronizer with
// anti-detect jitter: actions on one Master device fan out concurrently to
// selected Follower devices using normalized (0..1) coordinates, with optional
// per-follower random delay and pixel offset so platforms do not flag farm sync.
package sync

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"
)

// Target is one Follower device that can receive synchronized input events.
type Target interface {
	ID() string
	ScreenSize() (w, h int)
	Tap(ctx context.Context, x, y int) error
	Swipe(ctx context.Context, x1, y1, x2, y2, ms int) error
	PressKey(ctx context.Context, key string) error
}

// Jitter controls anti-detect randomization applied independently per Follower.
type Jitter struct {
	Enabled    bool `json:"enabled"`
	DelayMinMS int  `json:"delayMinMs"`
	DelayMaxMS int  `json:"delayMaxMs"`
	OffsetPX   int  `json:"offsetPx"`
}

// Status is the live synchronizer state exposed to the UI.
type Status struct {
	Active    bool     `json:"active"`
	MasterID  string   `json:"masterId"`
	Followers []string `json:"followers"`
	Jitter    Jitter   `json:"jitter"`
}

// Hub manages one active Master -> Followers synchronization session.
type Hub struct {
	mu        sync.RWMutex
	masterID  string
	followers map[string]Target
	jitter    Jitter
}

func NewHub() *Hub {
	return &Hub{followers: map[string]Target{}}
}

// Start activates sync from masterID to followers. Default jitter is 40..220 ms
// and ±5 px when enabled with zero bounds.
func (h *Hub) Start(masterID string, followers []Target, j Jitter) {
	if j.Enabled {
		if j.DelayMinMS <= 0 {
			j.DelayMinMS = 40
		}
		if j.DelayMaxMS < j.DelayMinMS {
			j.DelayMaxMS = max(j.DelayMinMS+160, 220)
		}
		if j.OffsetPX <= 0 {
			j.OffsetPX = 5
		}
	}
	h.mu.Lock()
	h.masterID = masterID
	h.jitter = j
	h.followers = make(map[string]Target, len(followers))
	for _, f := range followers {
		if f != nil && f.ID() != masterID {
			h.followers[f.ID()] = f
		}
	}
	h.mu.Unlock()
}

// Stop ends the active synchronization session.
func (h *Hub) Stop() {
	h.mu.Lock()
	h.masterID = ""
	h.followers = map[string]Target{}
	h.mu.Unlock()
}

// Status returns a snapshot of the current sync configuration.
func (h *Hub) Status() Status {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]string, 0, len(h.followers))
	for id := range h.followers {
		ids = append(ids, id)
	}
	return Status{
		Active:    h.masterID != "" && len(ids) > 0,
		MasterID:  h.masterID,
		Followers: ids,
		Jitter:    h.jitter,
	}
}

func (h *Hub) snapshot(masterID string) ([]Target, Jitter) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.masterID == "" || h.masterID != masterID || len(h.followers) == 0 {
		return nil, Jitter{}
	}
	out := make([]Target, 0, len(h.followers))
	for _, f := range h.followers {
		out = append(out, f)
	}
	return out, h.jitter
}

func jitterDelay(j Jitter) time.Duration {
	if !j.Enabled || j.DelayMaxMS <= 0 {
		return 0
	}
	span := j.DelayMaxMS - j.DelayMinMS
	ms := j.DelayMinMS
	if span > 0 {
		ms += rand.IntN(span + 1)
	}
	return time.Duration(ms) * time.Millisecond
}

func jitterCoord(v, limit int, j Jitter) int {
	if j.Enabled && j.OffsetPX > 0 {
		v += rand.IntN(2*j.OffsetPX+1) - j.OffsetPX
	}
	if v < 0 {
		return 0
	}
	if limit > 0 && v >= limit {
		return limit - 1
	}
	return v
}

// DispatchTap mirrors a normalized tap (rx, ry in 0..1) from masterID to all followers.
func (h *Hub) DispatchTap(ctx context.Context, masterID string, rx, ry float64) {
	targets, j := h.snapshot(masterID)
	for _, t := range targets {
		go func(dst Target) {
			if d := jitterDelay(j); d > 0 {
				select {
				case <-time.After(d):
				case <-ctx.Done():
					return
				}
			}
			w, ht := dst.ScreenSize()
			x := jitterCoord(int(rx*float64(w)), w, j)
			y := jitterCoord(int(ry*float64(ht)), ht, j)
			_ = dst.Tap(ctx, x, y)
		}(t)
	}
}

// DispatchSwipe mirrors a normalized swipe from masterID to all followers.
func (h *Hub) DispatchSwipe(ctx context.Context, masterID string, rx1, ry1, rx2, ry2 float64, ms int) {
	targets, j := h.snapshot(masterID)
	for _, t := range targets {
		go func(dst Target) {
			if d := jitterDelay(j); d > 0 {
				select {
				case <-time.After(d):
				case <-ctx.Done():
					return
				}
			}
			w, ht := dst.ScreenSize()
			x1 := jitterCoord(int(rx1*float64(w)), w, j)
			y1 := jitterCoord(int(ry1*float64(ht)), ht, j)
			x2 := jitterCoord(int(rx2*float64(w)), w, j)
			y2 := jitterCoord(int(ry2*float64(ht)), ht, j)
			_ = dst.Swipe(ctx, x1, y1, x2, y2, ms)
		}(t)
	}
}

// DispatchKey mirrors a hardware key press from masterID to all followers.
func (h *Hub) DispatchKey(ctx context.Context, masterID, key string) {
	targets, j := h.snapshot(masterID)
	for _, t := range targets {
		go func(dst Target) {
			if d := jitterDelay(j); d > 0 {
				select {
				case <-time.After(d):
				case <-ctx.Done():
					return
				}
			}
			_ = dst.PressKey(ctx, key)
		}(t)
	}
}
