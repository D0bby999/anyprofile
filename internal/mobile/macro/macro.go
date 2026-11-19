// SPDX-License-Identifier: AGPL-3.0-or-later

// Package macro implements a MuMu-style Operation Recorder for AnyMobile:
// records normalized touch/swipe/key sequences on a device into JSON files and
// replays them on one or many devices with configurable loops and speed.
package macro

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Step is one recorded action. Coordinates are normalized (0..1) so a macro
// recorded on 780x1688 replays accurately on 540x1170 or 1170x2532.
type Step struct {
	Kind       string  `json:"kind"` // "tap", "swipe", "key"
	DelayMS    int64   `json:"delayMs"`
	RX         float64 `json:"rx,omitempty"`
	RY         float64 `json:"ry,omitempty"`
	RX2        float64 `json:"rx2,omitempty"`
	RY2        float64 `json:"ry2,omitempty"`
	DurationMS int     `json:"durationMs,omitempty"`
	Key        string  `json:"key,omitempty"`
}

// Script is a saved macro file on disk.
type Script struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	Steps     []Step `json:"steps"`
}

// PlayerTarget is a running device that can execute macro steps.
type PlayerTarget interface {
	ScreenSize() (w, h int)
	Tap(ctx context.Context, x, y int) error
	Swipe(ctx context.Context, x1, y1, x2, y2, ms int) error
	PressKey(ctx context.Context, key string) error
}

// PlayOptions controls macro playback.
type PlayOptions struct {
	Loops int     `json:"loops"` // 1 = once; <=0 defaults to 1
	Speed float64 `json:"speed"` // 1.0 = normal, 2.0 = 2x faster
}

// Store persists macro scripts as JSON files inside Dir.
type Store struct {
	Dir string
}

func validID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func (s Store) path(id string) (string, error) {
	if !validID(id) {
		return "", fmt.Errorf("invalid macro id %q", id)
	}
	return filepath.Join(s.Dir, id+".json"), nil
}

func (s Store) Save(sc Script) error {
	p, err := s.path(sc.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".part"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (s Store) Load(id string) (Script, error) {
	p, err := s.path(id)
	if err != nil {
		return Script{}, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return Script{}, err
	}
	var sc Script
	return sc, json.Unmarshal(b, &sc)
}

func (s Store) Delete(id string) error {
	p, err := s.path(id)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

func (s Store) List() ([]Script, error) {
	entries, err := os.ReadDir(s.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Script
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.Dir, e.Name()))
		if err != nil {
			continue
		}
		var sc Script
		if json.Unmarshal(b, &sc) == nil && sc.ID != "" {
			out = append(out, sc)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

// Recorder captures steps from one device until Stop is called.
type Recorder struct {
	mu       sync.Mutex
	deviceID string
	name     string
	lastAt   time.Time
	steps    []Step
}

func (r *Recorder) Start(deviceID, name string) {
	r.mu.Lock()
	r.deviceID = deviceID
	r.name = strings.TrimSpace(name)
	r.lastAt = time.Now()
	r.steps = nil
	r.mu.Unlock()
}

func (r *Recorder) ActiveDevice() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.deviceID
}

func (r *Recorder) Record(deviceID string, st Step) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deviceID == "" || r.deviceID != deviceID {
		return
	}
	now := time.Now()
	st.DelayMS = now.Sub(r.lastAt).Milliseconds()
	r.lastAt = now
	r.steps = append(r.steps, st)
}

func (r *Recorder) Stop(id string) (Script, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deviceID == "" {
		return Script{}, false
	}
	name := r.name
	if name == "" {
		name = "Macro " + time.Now().Format("15:04:05")
	}
	sc := Script{
		ID:        id,
		Name:      name,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Steps:     append([]Step(nil), r.steps...),
	}
	r.deviceID, r.name, r.steps = "", "", nil
	return sc, true
}

// Play runs sc on target according to opts until complete or ctx is cancelled.
func Play(ctx context.Context, sc Script, target PlayerTarget, opts PlayOptions) error {
	loops := opts.Loops
	if loops <= 0 {
		loops = 1
	}
	speed := opts.Speed
	if speed <= 0 {
		speed = 1.0
	}
	for l := 0; l < loops; l++ {
		for _, st := range sc.Steps {
			if st.DelayMS > 0 {
				wait := time.Duration(float64(st.DelayMS)/speed) * time.Millisecond
				if wait > 0 {
					select {
					case <-time.After(wait):
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}
			w, h := target.ScreenSize()
			var err error
			switch st.Kind {
			case "tap":
				err = target.Tap(ctx, int(st.RX*float64(w)), int(st.RY*float64(h)))
			case "swipe":
				dur := st.DurationMS
				if dur <= 0 {
					dur = 250
				}
				err = target.Swipe(ctx,
					int(st.RX*float64(w)), int(st.RY*float64(h)),
					int(st.RX2*float64(w)), int(st.RY2*float64(h)), dur)
			case "key":
				err = target.PressKey(ctx, st.Key)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}
