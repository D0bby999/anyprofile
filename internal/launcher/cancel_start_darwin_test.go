// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

type cancelSpawnEngine struct {
	stubEngine
	pidFile string
}

func (e cancelSpawnEngine) BuildLaunch(context.Context, store.Profile, engine.Runtime) (engine.LaunchSpec, error) {
	// Deliberately never creates SingletonLock, so cancellation happens after
	// spawn but before startup completes. The child tests process-group cleanup.
	return engine.LaunchSpec{Binary: "/bin/sh", Args: []string{"-c", `echo $$ > "$1"; sleep 30 & echo $! > "$2"; wait`, "startup-test", e.pidFile, e.pidFile + ".child"}}, nil
}

func TestCancelStartupKillsProcessTree(t *testing.T) {
	dirs := paths.Dirs{Root: t.TempDir()}
	if err := dirs.Ensure(); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dirs.DB(), dirs.Backups())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p := &store.Profile{ID: "cancel", Name: "Cancel", Engine: "stub", EngineVersion: "1.0", PersonaID: "x", SeedHex: "00112233aabbccdd", Platform: "macos", Timezone: "UTC", Languages: "en-US", UILocale: "en-US"}
	if err := st.CreateProfile(p); err != nil {
		t.Fatal(err)
	}
	e := cancelSpawnEngine{pidFile: filepath.Join(dirs.Root, "startup.pid")}
	l := New(st, secrets.NewMemory(), dirs, []engine.Engine{e}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	l.SetMaxRunning(1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- l.StartBehind(ctx, p.ID) }()
	var pids []int
	for _, path := range []string{e.pidFile, e.pidFile + ".child"} {
		for ctx.Err() == nil {
			raw, err := os.ReadFile(path)
			if err == nil {
				if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil {
					pids = append(pids, pid)
					break
				}
			}
			time.Sleep(time.Millisecond)
		}
	}
	if len(pids) != 2 {
		t.Fatal("helper process did not start")
	}
	defer func() {
		for _, pid := range pids {
			if alive(pid) {
				killTree(pid)
			}
		}
	}()
	if err := l.Stop(p.ID); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("start=%v", err)
	}
	for _, pid := range pids {
		deadline := time.Now().Add(2 * time.Second)
		for alive(pid) && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		if alive(pid) {
			t.Errorf("startup process %d survived cancellation", pid)
		}
	}
	if len(l.List()) != 0 || l.gate.Running() != 0 {
		t.Fatal("startup cancellation leaked proc/slot")
	}
	launches, err := st.ListLaunches()
	if err != nil || len(launches) != 0 {
		t.Fatalf("stale launch records: %+v %v", launches, err)
	}
}
