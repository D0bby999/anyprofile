// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package launcher

// Runs the real Fury core and Apostate. Needs:
//   ANYPROFILE_TEST_FURY_APP      path to an extracted, verified Fury.app (fury.Version)
//   tools/fury-config built       (make fury-config)
//   ANYPROFILE_TEST_APOSTATE_APP  path to Apostate 0.5.1: its Chromium.app on
//                                 macOS, the folder holding chrome.exe on Windows
// Fury runs on macOS only; its tests skip without ANYPROFILE_TEST_FURY_APP.

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/D0bby999/anyprofile/internal/cookies"
	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/engine/apostate"
	"github.com/D0bby999/anyprofile/internal/engine/fury"
	"github.com/D0bby999/anyprofile/internal/fingerprint"
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
	root, err := os.MkdirTemp(shortTemp(), "apit")
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
	l := New(r.st, r.sec, r.dirs, []engine.Engine{r.eng}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	l.TestArgs = headlessArgs()
	return l
}

// headlessArgs runs the browsers without a window when
// ANYPROFILE_TEST_HEADLESS is set: tests then leave the screen alone.
func headlessArgs() []string {
	if os.Getenv("ANYPROFILE_TEST_HEADLESS") == "" {
		return nil
	}
	return []string{"--headless=new"}
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
			if err := l.StartBehind(context.Background(), id); err != nil {
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
	if err := l.StartBehind(context.Background(), infos[0].ProfileID); err != nil {
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
	if out := processesUsing(t, r.dirs.Root); out != "" {
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
	if err := l.StartBehind(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	waitVerdict("baseline", 20*time.Second)
	requireChecksPass(t, r.st, id)
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
	if err := l.StartBehind(context.Background(), id); err != nil {
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
	if err := l.StartBehind(context.Background(), id); err != nil {
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
	if err := l.StartBehind(context.Background(), id); err != nil {
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
	if !alive(pid) {
		t.Fatal("browser died after the app closed its pipe")
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

	if err := l.StartBehind(context.Background(), id); err != nil {
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
	if err := l.StartBehind(context.Background(), id); err != ErrCookieKeyLost {
		t.Fatalf("lost key err = %v", err)
	}
}

func newApostateRig(t *testing.T) *rig {
	app := os.Getenv("ANYPROFILE_TEST_APOSTATE_APP")
	if app == "" {
		t.Skip("ANYPROFILE_TEST_APOSTATE_APP not set")
	}
	root, err := os.MkdirTemp(shortTemp(), "apit")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	dirs := paths.Dirs{Root: root}
	if err := dirs.Ensure(); err != nil {
		t.Fatal(err)
	}
	eng := apostate.New(dirs)
	os.MkdirAll(filepath.Dir(eng.AppPath()), 0o700)
	if err := os.Symlink(app, eng.AppPath()); err != nil {
		t.Fatal(err)
	}
	if err := eng.Complete(); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dirs.DB(), dirs.Backups())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	eng.MockKeychain = true // a test must never raise a Keychain dialog
	return &rig{dirs: dirs, st: st, sec: secrets.NewMemory(), eng: eng}
}

func TestApostateLifecycle(t *testing.T) {
	r := newApostateRig(t)
	l := r.launcher(t)
	l.Capture = true
	const id = "ap1"
	if err := r.st.CreateProfile(&store.Profile{ID: id, Name: "Apostate", Engine: "apostate",
		EngineVersion: apostate.Version, PersonaID: apostate.HostPersona, Platform: apostate.HostPlatform,
		SeedHex: "00000000000a11ce", Timezone: "Asia/Taipei", Languages: "zh-TW,zh,en", UILocale: "zh-TW",
		EngineMeta: store.EngineMeta{HWC: 4, MemGiB: 8}}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	waitVerdict := func(want string) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			if p, err := r.st.GetProfile(id); err == nil && p.FPVerdict == want {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
		p, _ := r.st.GetProfile(id)
		t.Fatalf("verdict never became %q: %+v", want, p)
	}
	conn := func() (*proc, int) {
		l.mu.Lock()
		defer l.mu.Unlock()
		return l.procs[id], l.procs[id].info.PID
	}

	// Launch 1: baseline with the pinned values, no TCP listener.
	if err := l.StartBehind(ctx, id); err != nil {
		t.Fatal(err)
	}
	waitVerdict("baseline")
	db, err := sql.Open("sqlite", "file:"+r.dirs.DB())
	if err != nil {
		t.Fatal(err)
	}
	var last string
	if err := db.QueryRow(`SELECT fp_last FROM profiles WHERE id = ?`, id).Scan(&last); err != nil {
		t.Fatal(err)
	}
	db.Close()
	var s fingerprint.Sample
	if err := json.Unmarshal([]byte(last), &s); err != nil {
		t.Fatal(err)
	}
	// HWC is pinned at 4 and clamped to the host's cores, as the app does.
	if s.HWC != min(4, runtime.NumCPU()) || s.TZ != "Asia/Taipei" || !fingerprint.LangsMatch(s.Langs, "zh-TW,zh,en", true) ||
		s.Platform != fingerprint.NavPlatform(apostate.HostPlatform) || !strings.Contains(s.UA, "Chrome/155.") {
		t.Fatalf("sample = %+v", s)
	}
	if ss, _ := r.st.Sessions(id, 10); len(ss) != 1 || ss[0].StoppedAt != "" || ss[0].Engine != "apostate" {
		t.Fatalf("sessions after launch 1 = %+v", ss)
	}
	requireChecksPass(t, r.st, id)
	p1, pid := conn()
	if ports := listeningPorts(t, pid); ports != "" {
		t.Fatalf("launch listens: %s", ports)
	}
	expires := time.Now().Add(24 * time.Hour).Unix()
	if _, err := p1.cdp.Call(ctx, "", "Storage.setCookies", map[string]any{"cookies": []map[string]any{{
		"name": "ap_it", "value": "1", "domain": "example.com", "path": "/", "expires": expires, "secure": true}}}); err != nil {
		t.Fatal(err)
	}
	if err := l.Stop(id); err != nil {
		t.Fatal(err)
	}
	if ss, _ := r.st.Sessions(id, 10); len(ss) != 1 || ss[0].StoppedAt == "" {
		t.Fatalf("session not closed after stop: %+v", ss)
	}

	// Launch 2: same fingerprint, cookie kept.
	if err := l.StartBehind(ctx, id); err != nil {
		t.Fatal(err)
	}
	waitVerdict("match")
	p2, pid := conn()
	raw, err := p2.cdp.Call(ctx, "", "Storage.getCookies", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"name":"ap_it"`) {
		t.Fatalf("cookie lost after restart: %.300s", raw)
	}

	// The browser survives the app closing its pipe ends (keep-alive copies
	// sit on fds 5 and 6: Apostate has no data pipes).
	p2.cdp.Close()
	time.Sleep(3 * time.Second)
	if !alive(pid) {
		t.Fatal("browser died after the app closed its pipe")
	}
	if err := l.Stop(id); err != nil {
		t.Fatal(err)
	}
	udd := r.dirs.Profile(id)
	if left := processesUsing(t, udd); left != "" {
		t.Fatalf("processes left for %s: %s", udd, left)
	}
}

// requireChecksPass fails the test with every check that did not pass on the
// profile's newest session.
func requireChecksPass(t *testing.T, st *store.Store, id string) {
	t.Helper()
	ss, err := st.Sessions(id, 1)
	if err != nil || len(ss) == 0 {
		t.Fatalf("no session: %v", err)
	}
	var checks []fingerprint.Check
	if err := json.Unmarshal(ss[0].Checks, &checks); err != nil || len(checks) == 0 {
		t.Fatalf("session has no checks: %s %v", ss[0].Checks, err)
	}
	for _, c := range checks {
		if !c.OK {
			t.Errorf("check %s failed: got %q, want %q", c.ID, c.Got, c.Want)
		}
	}
	t.Logf("%d checks passed", len(checks))
}

func TestApostateCookiesInAndOut(t *testing.T) {
	r := newApostateRig(t)
	l := r.launcher(t)
	l.Capture = true
	const id = "ap2"
	if err := r.st.CreateProfile(&store.Profile{ID: id, Name: "Cookies", Engine: "apostate",
		EngineVersion: apostate.Version, PersonaID: apostate.HostPersona, Platform: apostate.HostPlatform,
		SeedHex: "00000000000b0b0b", Timezone: "UTC", Languages: "en-US,en", UILocale: "en-US"}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := l.StartBehind(ctx, id); err != nil {
		t.Fatal(err)
	}
	defer l.Stop(id)
	wctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	conn, err := l.WaitConn(wctx, id)
	cancel()
	if err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(t.TempDir(), "cookies.txt")
	os.WriteFile(file, []byte("#HttpOnly_.example.com\tTRUE\t/\tTRUE\t4102444800\tit_auth\tt0k\n"+
		".example.com\tTRUE\t/\tFALSE\t4102444800\tit_lang\ten\n"), 0o600)
	b, _ := os.ReadFile(file)
	cs, _, err := cookies.Parse(b)
	if err != nil || len(cs) != 2 {
		t.Fatalf("parse = %d %v", len(cs), err)
	}
	if _, err := conn.Call(ctx, "", "Storage.setCookies", map[string]any{"cookies": cs}); err != nil {
		t.Fatal(err)
	}
	raw, err := conn.Call(ctx, "", "Storage.getCookies", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	var got struct{ Cookies []cookies.Cookie }
	json.Unmarshal(raw, &got)
	var mine []cookies.Cookie
	for _, c := range got.Cookies {
		if strings.HasPrefix(c.Name, "it_") {
			mine = append(mine, c)
		}
	}
	if len(mine) != 2 {
		t.Fatalf("cookies in the browser = %+v", mine)
	}
	for _, c := range mine {
		if c.Name == "it_auth" && !c.HTTPOnly {
			t.Fatal("HttpOnly lost")
		}
	}

	out, err := cookies.Export(mine)
	if err != nil {
		t.Fatal(err)
	}
	exported := filepath.Join(t.TempDir(), "out.json")
	if err := os.WriteFile(exported, out, 0o600); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(exported); runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 {
		t.Fatalf("export mode = %v", st.Mode().Perm())
	}
	back, _, err := cookies.Parse(out)
	if err != nil || len(back) != 2 {
		t.Fatalf("re-parse = %d %v", len(back), err)
	}
}
