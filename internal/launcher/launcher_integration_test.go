// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package launcher

// Runs the real Fury core. Needs:
//   ANYPROFILE_TEST_FURY_APP  path to an extracted, verified Fury.app (0.3.5)
//   tools/fury-config built   (make fury-config)

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/engine/fury"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

type rig struct {
	dirs paths.Dirs
	st   *store.Store
	sec  *secrets.Memory
	eng  engine.Engine
}

func newRig(t *testing.T) *rig {
	app := os.Getenv("ANYPROFILE_TEST_FURY_APP")
	if app == "" {
		t.Skip("ANYPROFILE_TEST_FURY_APP not set")
	}
	cfgBin, _ := filepath.Abs("../../tools/fury-config/target/release/fury-config")
	// Short root: Chromium's SingletonSocket path must fit in sun_path.
	root, err := os.MkdirTemp("/tmp", "apit")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	dirs := paths.Dirs{Root: root}
	if err := dirs.Ensure(); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(dirs.Engine("fury", fury.Version), 0o700)
	if err := os.Symlink(app, filepath.Join(dirs.Engine("fury", fury.Version), "Fury.app")); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dirs.DB(), dirs.Backups())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return &rig{dirs: dirs, st: st, sec: secrets.NewMemory(), eng: fury.New(dirs, cfgBin)}
}

func (r *rig) launcher(t *testing.T) *Launcher {
	return New(r.st, r.sec, r.dirs, []engine.Engine{r.eng}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func (r *rig) profile(t *testing.T, i int) string {
	id := "it" + strconv.Itoa(i)
	ident, err := r.eng.NewIdentity(context.Background(), uint64(i)*0x9e3779b97f4a7c15)
	if err != nil {
		t.Fatal(err)
	}
	p := &store.Profile{ID: id, Name: "Kenh " + strconv.Itoa(i), Engine: "fury", EngineVersion: fury.Version,
		PersonaID: ident.PersonaID, SeedHex: "00000000000000" + strconv.Itoa(10+i), Platform: ident.Platform,
		Timezone: "Asia/Ho_Chi_Minh", Languages: "en-US,en", UILocale: "en-US"}
	if err := r.st.CreateProfile(p); err != nil {
		t.Fatal(err)
	}
	return id
}

func listeningPorts(t *testing.T, pid int) string {
	out, _ := exec.Command("lsof", "-nP", "-a", "-iTCP", "-sTCP:LISTEN", "-g", strconv.Itoa(pid)).Output()
	return strings.TrimSpace(string(out))
}

func TestLifecycleFiveProfiles(t *testing.T) {
	r := newRig(t)
	l := r.launcher(t)
	ids := make([]string, 5)
	for i := range ids {
		ids[i] = r.profile(t, i)
	}

	var wg sync.WaitGroup
	durations := make([]time.Duration, len(ids))
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			t0 := time.Now()
			if err := l.Start(context.Background(), id); err != nil {
				t.Errorf("start %s: %v", id, err)
			}
			durations[i] = time.Since(t0)
		}(i, id)
	}
	wg.Wait()
	t.Logf("start durations: %v", durations)
	infos := l.List()
	if len(infos) != 5 {
		t.Fatalf("running = %d", len(infos))
	}

	// Second Start on an open profile must not spawn another browser.
	pid0 := infos[0].PID
	if err := l.Start(context.Background(), infos[0].ProfileID); err != nil {
		t.Fatal(err)
	}
	if n := len(l.List()); n != 5 {
		t.Fatalf("after second start: %d", n)
	}

	for _, in := range infos {
		if ports := listeningPorts(t, in.PID); ports != "" {
			t.Errorf("profile %s listens without diagnostics:\n%s", in.ProfileID, ports)
		}
		args, _ := exec.Command("ps", "-o", "command=", "-p", strconv.Itoa(in.PID)).Output()
		for _, bad := range []string{"schema_version", "mock-keychain", "remote-allow-origins", "remote-debugging"} {
			if strings.Contains(string(args), bad) {
				t.Errorf("argv of %d contains %q", in.PID, bad)
			}
		}
	}

	// Simulated crash: a fresh launcher on the same store adopts all five.
	l2 := r.launcher(t)
	l2.Adopt()
	if n := len(l2.List()); n != 5 {
		t.Fatalf("adopted = %d", n)
	}
	l2.StopAll()
	time.Sleep(time.Second)
	for _, in := range infos {
		if alive(in.PID) {
			t.Errorf("pid %d survived StopAll", in.PID)
		}
	}
	if out, _ := exec.Command("pgrep", "-f", r.dirs.Root).Output(); len(out) > 0 {
		t.Errorf("leftover processes: %s", out)
	}
	if ls, _ := r.st.ListLaunches(); len(ls) != 0 {
		t.Errorf("launch rows left: %+v", ls)
	}
	_ = pid0
}

func TestFingerprintCheck(t *testing.T) {
	r := newRig(t)
	l := r.launcher(t)
	l.Capture = true
	id := r.profile(t, 20)

	waitVerdict := func(want string, timeout time.Duration) {
		t.Helper()
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			if p, err := r.st.GetProfile(id); err == nil && p.FPVerdict == want {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
		p, _ := r.st.GetProfile(id)
		t.Fatalf("verdict never became %q: %+v", want, p)
	}

	// Launch 1: baseline, no TCP listeners.
	if err := l.Start(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	waitVerdict("baseline", 20*time.Second)
	l.mu.Lock()
	pid := l.procs[id].info.PID
	l.mu.Unlock()
	if ports := listeningPorts(t, pid); ports != "" {
		t.Fatalf("baseline launch listens: %s", ports)
	}
	if err := l.Stop(id); err != nil {
		t.Fatal(err)
	}

	// Launch 2: match.
	if err := l.Start(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	waitVerdict("match", 20*time.Second)
	if err := l.Stop(id); err != nil {
		t.Fatal(err)
	}

	// Change the seed: the next launch must drift.
	db, err := sql.Open("sqlite", "file:"+r.dirs.DB())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE profiles SET seed_hex = '99887766aabbccdd' WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if err := l.Start(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	waitVerdict("drift", 20*time.Second)
	p, _ := r.st.GetProfile(id)
	found := false
	for _, d := range p.FPDiff {
		if d == "canvas" {
			found = true
		}
	}
	if !found {
		t.Fatalf("drift fields = %v, want canvas", p.FPDiff)
	}
	if err := l.Stop(id); err != nil {
		t.Fatal(err)
	}

	// Launch 4: the browser survives the app closing its pipe ends.
	// fp_checked_at has one-second resolution, so clear it rather than wait
	// for it to change: launches 3 and 4 can be measured within one second.
	db, err = sql.Open("sqlite", "file:"+r.dirs.DB())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE profiles SET fp_checked_at = '' WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if err := l.Start(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		p, _ := r.st.GetProfile(id)
		if p.FPCheckedAt != "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no new measurement")
		}
		time.Sleep(200 * time.Millisecond)
	}
	l.mu.Lock()
	proc := l.procs[id]
	l.mu.Unlock()
	proc.cdp.Close()
	time.Sleep(3 * time.Second)
	l.mu.Lock()
	pid = l.procs[id].info.PID
	l.mu.Unlock()
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatalf("browser died after the app closed its pipe: %v", err)
	}
	if err := l.Stop(id); err != nil {
		t.Fatal(err)
	}
}

func TestCookieKeyAndDiagnostics(t *testing.T) {
	r := newRig(t)
	l := r.launcher(t)
	id := r.profile(t, 7)
	p, _ := r.st.GetProfile(id)
	p.CDPNextLaunch = true
	r.st.UpdateProfile(p)

	if err := l.Start(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	in := l.List()[0]
	if in.CDPPort == 0 {
		t.Fatal("diagnostics requested but no DevTools port")
	}
	if p, _ := r.st.GetProfile(id); p.CDPNextLaunch {
		t.Fatal("diagnostics must reset after one launch")
	}
	if err := l.Stop(id); err != nil {
		t.Fatal(err)
	}

	// Key lost from the Keychain: refuse instead of discarding the cookie jar.
	r.sec.Delete(secrets.ServiceCookieKey, id)
	if err := l.Start(context.Background(), id); err != ErrCookieKeyLost {
		t.Fatalf("lost key err = %v", err)
	}
}
