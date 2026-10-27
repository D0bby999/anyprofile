// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

func waitForQueue(t *testing.T, g *launchGate, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if g.Waiting() == n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("waiting=%d, want %d", g.Waiting(), n)
}

func TestGateRejectsAlreadyCancelledContext(t *testing.T) {
	for _, ceiling := range []int{0, 1} {
		g := newLaunchGate()
		g.SetMax(ceiling)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := g.Acquire(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled acquire: %v", err)
		}
		if g.Running() != 0 || g.Waiting() != 0 {
			t.Fatal("cancellation leaked a slot or waiter")
		}
	}
}

func TestGateAdoptCountsAndFIFO(t *testing.T) {
	g := newLaunchGate()
	g.SetMax(1)
	g.Adopt()
	order := make(chan int, 3)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i := 0; i < 3; i++ {
		go func(i int) {
			if err := g.Acquire(ctx); err == nil {
				order <- i
			}
		}(i)
		waitForQueue(t, g, i+1)
	}
	if g.Running() != 1 {
		t.Fatal("adopted browser not counted")
	}
	for i := 0; i < 3; i++ {
		g.Release()
		select {
		case got := <-order:
			if got != i {
				t.Fatalf("FIFO got %d, want %d", got, i)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("waiter never admitted")
		}
	}
	g.Release()
	if g.Running() != 0 {
		t.Fatal("slot leak")
	}
}

func TestGateCancelledHeadDoesNotBlockNext(t *testing.T) {
	g := newLaunchGate()
	g.SetMax(1)
	g.Adopt()
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() { first <- g.Acquire(ctx) }()
	waitForQueue(t, g, 1)
	nextCtx, nextCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer nextCancel()
	second := make(chan error, 1)
	go func() { second <- g.Acquire(nextCtx) }()
	waitForQueue(t, g, 2)
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	g.Release()
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	g.Release()
}

func TestStopQueuedLaunchAndRejectDuplicate(t *testing.T) {
	l := New(nil, nil, paths.Dirs{}, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	l.SetMaxRunning(1)
	l.gate.Adopt()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- l.StartBehind(ctx, "queued") }()
	waitForQueue(t, l.gate, 1)
	if info, ok := l.Info("queued"); !ok || info.State != Queued {
		t.Fatalf("queued info=%+v, %v", info, ok)
	}
	if err := l.StartBehind(ctx, "queued"); err == nil {
		t.Fatal("duplicate queued request accepted")
	}
	if l.WaitingLaunches() != 1 {
		t.Fatal("duplicate created another waiter")
	}
	if err := l.Stop("queued"); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("start result=%v", err)
	}
	if _, ok := l.Info("queued"); ok || l.WaitingLaunches() != 0 {
		t.Fatal("stop left queued request behind")
	}
	l.gate.Release()
}

type queueTestEngine struct {
	stubEngine
	entered chan string
	release chan struct{}
}

func (e queueTestEngine) BuildLaunch(ctx context.Context, p store.Profile, _ engine.Runtime) (engine.LaunchSpec, error) {
	e.entered <- p.ID
	select {
	case <-ctx.Done():
		return engine.LaunchSpec{}, ctx.Err()
	case <-e.release:
		return engine.LaunchSpec{}, errors.New("test launch ended")
	}
}

func TestLauncherCeilingAdvancesAndStopAllCancels(t *testing.T) {
	dirs := paths.Dirs{Root: t.TempDir()}
	if err := dirs.Ensure(); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dirs.DB(), filepath.Join(dirs.Root, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	e := queueTestEngine{entered: make(chan string, 4), release: make(chan struct{})}
	l := New(st, secrets.NewMemory(), dirs, []engine.Engine{e}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	l.SetMaxRunning(2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 4)
	for _, id := range []string{"p1", "p2", "p3", "p4"} {
		p := &store.Profile{ID: id, Name: id, Engine: "stub", EngineVersion: "1.0", PersonaID: "x", SeedHex: "00112233aabbccdd", Platform: "macos", Timezone: "UTC", Languages: "en-US", UILocale: "en-US"}
		if err := st.CreateProfile(p); err != nil {
			t.Fatal(err)
		}
		go func(id string) { done <- l.StartBehind(ctx, id) }(id)
	}
	for range 2 {
		select {
		case <-e.entered:
		case <-time.After(2 * time.Second):
			t.Fatal("launch not admitted")
		}
	}
	waitForQueue(t, l.gate, 2)
	if l.gate.Running() != 2 {
		t.Fatal("ceiling not respected")
	}
	e.release <- struct{}{}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("first launch did not finish")
	}
	select {
	case <-e.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("queued launch did not advance")
	}
	waitForQueue(t, l.gate, 1)
	l.StopAll()
	for range 3 {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("StopAll left a launch blocked")
		}
	}
	if len(l.List()) != 0 || l.gate.Running() != 0 || l.WaitingLaunches() != 0 {
		t.Fatal("StopAll leaked a proc, waiter or slot")
	}
}
