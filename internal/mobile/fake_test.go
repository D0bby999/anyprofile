// SPDX-License-Identifier: AGPL-3.0-or-later

package mobile

import (
	"context"
	"errors"
	"sync"

	"github.com/D0bby999/anyprofile/internal/netproxy"
	"github.com/D0bby999/anyprofile/internal/store"
)

// fakeDriver runs nothing: a started device stays up until Stop or crash.
type fakeDriver struct {
	mu       sync.Mutex
	running  map[string]chan struct{}
	alive    map[string]bool // what Adopt finds still running
	startErr error
	opts     StartOptions
	adopted  []string
}

func newFake() *fakeDriver {
	return &fakeDriver{running: map[string]chan struct{}{}, alive: map[string]bool{}}
}

func (f *fakeDriver) Name() string     { return "vm" }
func (f *fakeDriver) Available() error { return nil }
func (f *fakeDriver) Create(ctx context.Context, d Device) (string, string, error) {
	return "dir-" + d.ID, "img-1", nil
}

func (f *fakeDriver) Start(ctx context.Context, d Device, o StartOptions) (Run, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.startErr != nil {
		return Run{}, f.startErr
	}
	f.opts = o
	done := make(chan struct{})
	f.running[d.ID] = done
	return Run{Launch: Launch{PID: 100, PIDStartTime: "t", ADBPort: 5555}, Done: done}, nil
}

func (f *fakeDriver) Stop(ctx context.Context, d Device) error {
	f.crash(d.ID)
	return nil
}

// crash ends the device as if it stopped by itself.
func (f *fakeDriver) crash(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if done, ok := f.running[id]; ok {
		close(done)
		delete(f.running, id)
	}
}

func (f *fakeDriver) Delete(ctx context.Context, d Device) error              { return nil }
func (f *fakeDriver) Backup(ctx context.Context, d Device, dir string) error  { return nil }
func (f *fakeDriver) Restore(ctx context.Context, d Device, dir string) error { return nil }

func (f *fakeDriver) Adopt(ctx context.Context, d Device, l Launch, up *netproxy.Upstream) (Run, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.alive[d.ID] {
		return Run{}, errors.New("gone")
	}
	f.adopted = append(f.adopted, d.ID)
	done := make(chan struct{})
	f.running[d.ID] = done
	return Run{Launch: l, Done: done}, nil
}

// memLaunches is mobile_launches in memory.
type memLaunches struct {
	mu sync.Mutex
	m  map[string]store.MobileLaunch
}

func newMem() *memLaunches { return &memLaunches{m: map[string]store.MobileLaunch{}} }

func (s *memLaunches) PutMobileLaunch(l store.MobileLaunch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[l.DeviceID] = l
	return nil
}

func (s *memLaunches) DeleteMobileLaunch(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}

func (s *memLaunches) ListMobileLaunches() ([]store.MobileLaunch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.MobileLaunch
	for _, l := range s.m {
		out = append(out, l)
	}
	return out, nil
}

func (s *memLaunches) has(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.m[id]
	return ok
}
