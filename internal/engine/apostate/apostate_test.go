// SPDX-License-Identifier: AGPL-3.0-or-later

package apostate

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/store"
)

var (
	_ engine.Engine               = (*Apostate)(nil)
	_ engine.PrimaryLanguageProbe = (*Apostate)(nil)
)

func testEngine(t *testing.T) *Apostate {
	a := New(paths.Dirs{Root: t.TempDir()})
	a.host = func() (int, int) { return 8, 16 }
	return a
}

func testProfile() store.Profile {
	return store.Profile{ID: "p1", Name: "Shop", Serial: 3, SeedHex: "00000000000a11ce",
		Timezone: "Asia/Taipei", Languages: "zh-TW,zh,en", EngineMeta: store.EngineMeta{HWC: 8, MemGiB: 16}}
}

func TestBuildLaunchProxy(t *testing.T) {
	a := testEngine(t)
	spec, err := a.BuildLaunch(context.Background(), testProfile(),
		engine.Runtime{UserDataDir: "/u", RelayPort: 41000, CDPPipe: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"--user-data-dir=/u", "--no-default-browser-check", "--no-first-run", "--restore-last-session",
		"--disable-background-networking", "--disable-domain-reliability", "--disable-breakpad",
		"--safebrowsing-disable-auto-update", "--disable-field-trial-config", "--disk-cache-size=20971520",
		"--window-name=#3 Shop", "--fingerprint=00000000000a11ce", "--fingerprint-platform=" + platform,
		"--fingerprint-timezone=Asia/Taipei", "--fingerprint-locale=zh-TW,zh,en",
		"--fingerprint-hardware-concurrency=8", "--fingerprint-device-memory=16",
		"--proxy-server=http://127.0.0.1:41000", "--proxy-bypass-list=<-loopback>", "--disable-quic",
		"--force-webrtc-ip-handling-policy=disable_non_proxied_udp", "--fingerprint-webrtc-udp=block",
		"--remote-debugging-pipe",
	}
	if !slices.Equal(spec.Args, want) {
		t.Fatalf("args =\n%s\nwant\n%s", strings.Join(spec.Args, "\n"), strings.Join(want, "\n"))
	}
	for _, bad := range []string{"--use-mock-keychain", "--remote-debugging-port", "--lang=", "--fingerprint-noise"} {
		for _, arg := range spec.Args {
			if strings.HasPrefix(arg, bad) {
				t.Fatalf("unexpected %s", arg)
			}
		}
	}
	wantEnv := []string{"GOOGLE_API_KEY=no", "GOOGLE_DEFAULT_CLIENT_ID=no", "GOOGLE_DEFAULT_CLIENT_SECRET=no"}
	if setsTZ {
		wantEnv = append([]string{"TZ=Asia/Taipei"}, wantEnv...)
	}
	if !slices.Equal(spec.Env, wantEnv) {
		t.Fatalf("env = %v", spec.Env)
	}
	if len(spec.Pipes) != 0 || !spec.CDPPipe || !strings.HasSuffix(spec.Binary, binRel) {
		t.Fatalf("spec = %+v", spec)
	}
}

func TestBuildLaunchNoProxy(t *testing.T) {
	spec, err := testEngine(t).BuildLaunch(context.Background(), testProfile(), engine.Runtime{UserDataDir: "/u"})
	if err != nil {
		t.Fatal(err)
	}
	for _, arg := range spec.Args {
		for _, bad := range []string{"--proxy-server", "--disable-quic", "--fingerprint-webrtc-udp", "--remote-debugging"} {
			if strings.HasPrefix(arg, bad) {
				t.Fatalf("unexpected %s without a proxy", arg)
			}
		}
	}
}

func TestBuildLaunchFreshStart(t *testing.T) {
	spec, err := testEngine(t).BuildLaunch(context.Background(), testProfile(),
		engine.Runtime{UserDataDir: "/u", FreshStart: true, StartURLs: []string{"https://a.example/"}})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(spec.Args, "--restore-last-session") {
		t.Fatal("a fresh start must not restore the last session")
	}
	if spec.Args[len(spec.Args)-1] != "https://a.example/" {
		t.Fatalf("start page not last: %v", spec.Args)
	}
}

func TestBuildLaunchExtensions(t *testing.T) {
	spec, err := testEngine(t).BuildLaunch(context.Background(), testProfile(),
		engine.Runtime{UserDataDir: "/u", Extensions: []string{"/lib/a", "/lib/b"}, StartURLs: []string{"https://a.example/"}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(spec.Args, "--load-extension=/lib/a,/lib/b") || spec.Args[len(spec.Args)-1] != "https://a.example/" {
		t.Fatalf("args = %v", spec.Args)
	}
	spec, _ = testEngine(t).BuildLaunch(context.Background(), testProfile(), engine.Runtime{UserDataDir: "/u"})
	for _, a := range spec.Args {
		if strings.HasPrefix(a, "--load-extension") {
			t.Fatalf("%s with no extension", a)
		}
	}
}

func TestBuildLaunchClampsHost(t *testing.T) {
	p := testProfile()
	p.EngineMeta = store.EngineMeta{HWC: 16, MemGiB: 64}
	spec, err := testEngine(t).BuildLaunch(context.Background(), p, engine.Runtime{UserDataDir: "/u"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(spec.Args, "--fingerprint-hardware-concurrency=8") || !slices.Contains(spec.Args, "--fingerprint-device-memory=16") {
		t.Fatalf("args = %v", spec.Args)
	}
}

func TestBuildLaunchBadSeed(t *testing.T) {
	p := testProfile()
	p.SeedHex = "zz"
	if _, err := testEngine(t).BuildLaunch(context.Background(), p, engine.Runtime{UserDataDir: "/u"}); err == nil {
		t.Fatal("bad seed accepted")
	}
}

func readPrefs(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func geo(m map[string]any) any {
	p, _ := m["profile"].(map[string]any)
	v, _ := p["default_content_setting_values"].(map[string]any)
	return v["geolocation"]
}

func TestPrepareProfileGeolocation(t *testing.T) {
	a := testEngine(t)
	udd := t.TempDir()
	prefs := filepath.Join(udd, "Default", "Preferences")
	if err := a.PrepareProfile(testProfile(), engine.Runtime{UserDataDir: udd}); err != nil {
		t.Fatal(err)
	}
	if g := geo(readPrefs(t, prefs)); g != float64(2) {
		t.Fatalf("new prefs geolocation = %v", g)
	}

	if err := os.WriteFile(prefs, []byte(`{"browser":{"x":1},"profile":{"name":"A"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := a.PrepareProfile(testProfile(), engine.Runtime{UserDataDir: udd}); err != nil {
		t.Fatal(err)
	}
	m := readPrefs(t, prefs)
	if g := geo(m); g != float64(2) {
		t.Fatalf("merged geolocation = %v", g)
	}
	if m["browser"].(map[string]any)["x"] != float64(1) || m["profile"].(map[string]any)["name"] != "A" {
		t.Fatalf("other keys lost: %v", m)
	}

	if err := os.WriteFile(prefs, []byte(`{`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := a.PrepareProfile(testProfile(), engine.Runtime{UserDataDir: udd}); err == nil {
		t.Fatal("broken prefs overwritten without error")
	}
	if b, _ := os.ReadFile(prefs); string(b) != "{" {
		t.Fatalf("broken prefs changed to %q", b)
	}
}

func TestBuildLaunchRefusesAnotherOS(t *testing.T) {
	p := testProfile()
	p.Platform = "windows"
	if platform == "windows" {
		p.Platform = "macos"
	}
	if _, err := testEngine(t).BuildLaunch(context.Background(), p, engine.Runtime{UserDataDir: "/u"}); !errors.Is(err, ErrOtherOS) {
		t.Fatalf("err = %v, want ErrOtherOS", err)
	}
	p.Platform = platform
	if _, err := testEngine(t).BuildLaunch(context.Background(), p, engine.Runtime{UserDataDir: "/u"}); err != nil {
		t.Fatal(err)
	}
}
