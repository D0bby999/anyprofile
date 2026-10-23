// SPDX-License-Identifier: AGPL-3.0-or-later

package mobile

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/netproxy"
	"github.com/D0bby999/anyprofile/internal/store"
)

func newManager(t *testing.T) (*Manager, *fakeDriver, *memLaunches) {
	t.Helper()
	f, st := newFake(), newMem()
	return NewManager(f, st, slog.New(slog.NewTextHandler(io.Discard, nil)), nil), f, st
}

// waitState polls until the device reaches want; a watcher goroutine sets it.
func waitState(t *testing.T, m *Manager, id string, want State) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for m.Info(id).State != want {
		if time.Now().After(deadline) {
			t.Fatalf("state %s, want %s", m.Info(id).State, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestStartStop(t *testing.T) {
	m, f, st := newManager(t)
	ctx := context.Background()
	d := Device{ID: "d1"}
	up := &netproxy.Upstream{Scheme: "socks5", Host: "h", Port: 1}
	if err := m.Start(ctx, d, StartOptions{Upstream: up, Timezone: "Asia/Tokyo", Locale: "ja-JP"}); err != nil {
		t.Fatal(err)
	}
	if got := m.Info("d1").State; got != Running {
		t.Fatalf("state %s", got)
	}
	if f.opts.Timezone != "Asia/Tokyo" || f.opts.Locale != "ja-JP" || f.opts.Upstream != up {
		t.Fatalf("driver got %+v", f.opts)
	}
	if !st.has("d1") || m.Running() != 1 {
		t.Fatal("launch not recorded")
	}
	if err := m.Start(ctx, d, StartOptions{}); err != nil {
		t.Fatalf("starting a running device: %v", err)
	}
	if err := m.Stop(ctx, d); err != nil {
		t.Fatal(err)
	}
	if got := m.Info("d1").State; got != Stopped {
		t.Fatalf("state after stop %s", got)
	}
	if st.has("d1") || m.Running() != 0 {
		t.Fatal("launch not forgotten")
	}
}

func TestStopsByItself(t *testing.T) {
	m, f, st := newManager(t)
	ctx := context.Background()
	if err := m.Start(ctx, Device{ID: "d1"}, StartOptions{}); err != nil {
		t.Fatal(err)
	}
	f.crash("d1")
	waitState(t, m, "d1", Stopped)
	if st.has("d1") {
		t.Fatal("launch kept after the device stopped")
	}
}

// A run's late watcher must not mark the next run of the same device stopped.
func TestRestartAfterStop(t *testing.T) {
	m, _, _ := newManager(t)
	ctx := context.Background()
	d := Device{ID: "d1"}
	for i := 0; i < 50; i++ {
		if err := m.Start(ctx, d, StartOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := m.Stop(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Start(ctx, d, StartOptions{}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if got := m.Info("d1").State; got != Running {
		t.Fatalf("state %s, want running", got)
	}
}

func TestStartFails(t *testing.T) {
	m, f, st := newManager(t)
	f.startErr = errors.New("qemu exited")
	err := m.Start(context.Background(), Device{ID: "d1"}, StartOptions{})
	if err == nil {
		t.Fatal("no error")
	}
	info := m.Info("d1")
	if info.State != Stopped || info.Error != "qemu exited" || st.has("d1") {
		t.Fatalf("after a failed start: %+v", info)
	}
}

func TestBusy(t *testing.T) {
	m, _, _ := newManager(t)
	ctx := context.Background()
	err := m.Busy("d1", Provisioning, func() error {
		if err := m.Start(ctx, Device{ID: "d1"}, StartOptions{}); !errors.Is(err, ErrBusy) {
			t.Errorf("start during work: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Info("d1").State; got != Stopped {
		t.Fatalf("state %s", got)
	}
}

func TestCreate(t *testing.T) {
	m, _, _ := newManager(t)
	ref, img, err := m.Create(context.Background(), Device{ID: "d1"})
	if err != nil || ref != "dir-d1" || img != "img-1" {
		t.Fatalf("%q %q %v", ref, img, err)
	}
	if got := m.Info("d1").State; got != Stopped {
		t.Fatalf("state %s", got)
	}
}

func TestAdopt(t *testing.T) {
	m, f, st := newManager(t)
	for _, id := range []string{"alive", "dead", "unknown"} {
		st.PutMobileLaunch(store.MobileLaunch{DeviceID: id, PID: 1})
	}
	f.alive["alive"] = true
	m.Adopt(context.Background(), map[string]AdoptDevice{
		"alive": {Device: Device{ID: "alive"}},
		"dead":  {Device: Device{ID: "dead"}},
	})
	if got := m.Info("alive").State; got != Running || !st.has("alive") {
		t.Fatalf("alive: %s", got)
	}
	if st.has("dead") || st.has("unknown") {
		t.Fatal("launches of devices that are gone were kept")
	}
	if len(f.adopted) != 1 {
		t.Fatalf("adopted %v", f.adopted)
	}
}
