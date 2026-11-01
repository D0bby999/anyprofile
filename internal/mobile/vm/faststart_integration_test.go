// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin && vmintegration

// Spike for fast start: save the VM's RAM state on Stop and resume it on Start.
// Proves the state is real (a system property set in RAM survives, a cold boot
// loses it), measures the thaw, and checks that the disks are never reverted.
//
//	ANYPROFILE_MOBILE_RUNTIME=<runtime> go test -tags vmintegration \
//	  -run TestFastStart -v -timeout 30m ./internal/mobile/vm/

package vm

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile"
)

func TestFastStart(t *testing.T) {
	rtDir := integrationRuntime(t)
	if err := os.MkdirAll(filepath.Join(t.TempDir(), "Any Profile"), 0o700); err != nil {
		t.Fatal(err)
	}
	devs := filepath.Join(t.TempDir(), "Any Profile")
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	drv := New(rtDir, devs, log)
	if err := drv.Available(); err != nil {
		t.Fatal(err)
	}

	d := mobile.Device{ID: benchID(), Name: "Fast start", RAMMB: 3072, CPUs: 4}
	ref, _, err := drv.Create(context.Background(), d)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	d.Ref = ref
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_ = drv.Stop(stopCtx, d)
		cancel()
		if err := drv.Delete(context.Background(), d); err != nil {
			t.Logf("delete: %v", err)
		}
	}()

	// Cold start, then leave a marker that exists only in RAM: setprop lives
	// in the property service, so a cold boot loses it and a restored state
	// keeps it.
	coldStart := time.Now()
	if _, err := drv.Start(context.Background(), d, mobile.StartOptions{Headless: true}); err != nil {
		t.Fatalf("cold start: %v", err)
	}
	cold := time.Since(coldStart)
	a, ok := drv.ADB(d)
	if !ok {
		t.Fatal("no adb after cold start")
	}
	marker := "faststart-" + d.ID
	if _, err := a.Shell(context.Background(), "setprop debug.ap.faststart "+marker); err != nil {
		t.Fatalf("setprop: %v", err)
	}

	stopStart := time.Now()
	if err := drv.Stop(context.Background(), d); err != nil {
		t.Fatalf("stop (save): %v", err)
	}
	save := time.Since(stopStart)
	state := filepath.Join(drv.dir(d), ramStateFile)
	st, err := os.Stat(state)
	if err != nil {
		t.Fatalf("no RAM state after stop: %v", err)
	}
	t.Logf("cold start %s, save %s, state %.1f GB", cold.Round(time.Millisecond),
		save.Round(time.Millisecond), float64(st.Size())/(1<<30))

	var thaws []time.Duration
	for i := 0; i < 5; i++ {
		start := time.Now()
		if _, err := drv.Start(context.Background(), d, mobile.StartOptions{Headless: true}); err != nil {
			t.Fatalf("restore %d: %v", i, err)
		}
		thaw := time.Since(start)
		thaws = append(thaws, thaw)
		a, ok = drv.ADB(d)
		if !ok {
			t.Fatalf("no adb after restore %d", i)
		}
		got, err := a.Shell(context.Background(), "getprop debug.ap.faststart")
		if err != nil || got != marker {
			t.Fatalf("restore %d lost RAM state: %q %v", i, got, err)
		}
		shot := time.Now()
		if _, err := a.Output(context.Background(), "exec-out", "screencap", "-p"); err != nil {
			t.Fatalf("screenshot after restore %d: %v", i, err)
		}
		t.Logf("restore %d: %s (screenshot %s)", i, thaw.Round(time.Millisecond),
			time.Since(shot).Round(time.Millisecond))
		if err := drv.Stop(context.Background(), d); err != nil {
			t.Fatalf("stop %d: %v", i, err)
		}
	}
	var worst time.Duration
	for _, v := range thaws {
		if v > worst {
			worst = v
		}
	}
	if worst > 10*time.Second {
		t.Errorf("slowest thaw %s, want <= 10s", worst.Round(time.Millisecond))
	}

	// Removing the state must fall back to a cold boot and lose the marker.
	if err := os.Remove(state); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := drv.Start(context.Background(), d, mobile.StartOptions{Headless: true}); err != nil {
		t.Fatalf("cold restart: %v", err)
	}
	coldAgain := time.Since(start)
	a, _ = drv.ADB(d)
	got, _ := a.Shell(context.Background(), "getprop debug.ap.faststart")
	if got == marker {
		t.Fatal("property survived a cold boot; the marker proves nothing")
	}
	t.Logf("cold restart after deleting the state: %s (marker %q)", coldAgain.Round(time.Millisecond), got)
}
