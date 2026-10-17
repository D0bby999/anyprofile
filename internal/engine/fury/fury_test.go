// SPDX-License-Identifier: AGPL-3.0-or-later

// Fury runs on macOS only (see engines_windows.go).

//go:build darwin

package fury

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/store"
	"github.com/ulikunitz/xz"
)

// configBinForTest is the helper built by `make fury-config`.
func configBinForTest(t *testing.T) string {
	p, _ := filepath.Abs("../../../tools/fury-config/target/release/fury-config")
	if _, err := os.Stat(p); err != nil {
		t.Skip("fury-config not built; run make fury-config")
	}
	return p
}

func sha(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func fakeCore(t *testing.T, binary string) []byte {
	var buf bytes.Buffer
	xw, _ := xz.NewWriter(&buf)
	tw := tar.NewWriter(xw)
	tw.WriteHeader(&tar.Header{Name: binRel, Typeflag: tar.TypeReg, Mode: 0o755, Size: int64(len(binary))})
	tw.Write([]byte(binary))
	tw.Close()
	xw.Close()
	return buf.Bytes()
}

// serveCore serves body over TLS as the only release in f's catalog, with
// the given hashes, and returns that release.
func serveCore(t *testing.T, f *Fury, body []byte, tarSHA, binSHA string) engine.Release {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(body) }))
	t.Cleanup(srv.Close)
	old := http.DefaultClient
	http.DefaultClient = srv.Client()
	t.Cleanup(func() { http.DefaultClient = old })
	r := Builtin()[0]
	r.URL, r.SHA256, r.Files = srv.URL+"/core.tar.xz", tarSHA, map[string]string{binRel: binSHA}
	f.SetCatalog(engine.NewCatalog("", r))
	return r
}

func TestInstallVerifiesThenStamps(t *testing.T) {
	core := fakeCore(t, "core-binary")
	dirs := paths.Dirs{Root: t.TempDir()}
	f := New(dirs, "/nonexistent")
	f.skipCodesign = true
	ctx := context.Background()

	// Wrong tarball hash: refused, nothing installed, nothing left behind.
	serveCore(t, f, core, strings.Repeat("0", 64), sha([]byte("core-binary")))
	if err := f.Install(ctx, Version, nil); err == nil {
		t.Fatal("bad tarball hash accepted")
	}
	if left, _ := filepath.Glob(filepath.Join(dirs.Engines(), "fury", "*")); len(left) != 0 {
		t.Fatalf("leftovers after failed install: %v", left)
	}

	// Wrong binary hash: refused too.
	serveCore(t, f, core, sha(core), strings.Repeat("1", 64))
	if err := f.Install(ctx, Version, nil); err == nil {
		t.Fatal("bad binary hash accepted")
	}

	r := serveCore(t, f, core, sha(core), sha([]byte("core-binary")))
	if err := f.Install(ctx, Version, nil); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(f.dir(Version), r.Bin)
	if _, err := os.Stat(bin); err != nil {
		t.Fatal(err)
	}
	if left, _ := filepath.Glob(filepath.Join(dirs.Engines(), "fury", "*.tar.xz")); len(left) != 0 {
		t.Fatalf("tarball kept: %v", left)
	}
	if st := f.Status(ctx, Version); !st.Installed {
		t.Fatalf("fresh install reported as %+v", st)
	}

	// One changed byte (same size, new mtime) must be caught.
	os.WriteFile(bin, []byte("core-binarY"), 0o755)
	if st := f.Status(ctx, Version); st.Installed || st.Problem == "" {
		t.Fatalf("tampered core reported as %+v", st)
	}
	if err := f.Remove(Version); err != nil {
		t.Fatal(err)
	}
	if st := f.Status(ctx, Version); st.Installed {
		t.Fatal("removed version still installed")
	}
	if st := f.Status(ctx, "9.9.9"); st.Problem == "" {
		t.Fatal("an unknown version has no problem")
	}
}

func testProfile() store.Profile {
	lat, lon := 40.71, -74.0
	return store.Profile{ID: "p1", Name: "Kenh 1", EngineVersion: Version, PersonaID: "macos-m4-1710x1112", SeedHex: "00112233aabbccdd",
		Timezone: "America/New_York", Languages: "en-US,en", UILocale: "en-US", Lat: &lat, Lon: &lon, Serial: 7}
}

func TestWindowName(t *testing.T) {
	if got := engine.WindowName(store.Profile{Name: "Shop A"}); got != "Shop A" {
		t.Fatalf("serial 0 name = %q", got)
	}
	if got := engine.WindowName(store.Profile{Name: "Shop A", Serial: 12}); got != "#12 Shop A" {
		t.Fatalf("serial 12 name = %q", got)
	}
}

func TestDeriveIsDeterministic(t *testing.T) {
	f := New(paths.Dirs{Root: t.TempDir()}, configBinForTest(t))
	a, err := f.deriveConfig(context.Background(), testProfile(), "155.0.8059.39")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := f.deriveConfig(context.Background(), testProfile(), "155.0.8059.39")
	if !bytes.Equal(a, b) || !bytes.Contains(a, []byte(`"schema_version":1`)) {
		t.Fatalf("derive not deterministic or missing schema_version (%d bytes)", len(a))
	}
	id, err := f.NewIdentity(context.Background(), 1<<63)
	if err != nil || !strings.HasPrefix(id.PersonaID, "macos-") || id.Platform != "macos" {
		t.Fatalf("identity = %+v %v", id, err)
	}
	l, err := f.LocaleFor(context.Background(), "VN")
	if err != nil || l.UILocale != "vi" || l.Languages[0] != "vi-VN" {
		t.Fatalf("locale = %+v %v", l, err)
	}
}

func TestBuildLaunchKeepsSecretsOffArgv(t *testing.T) {
	f := New(paths.Dirs{Root: t.TempDir()}, configBinForTest(t))
	key := bytes.Repeat([]byte{0xAB}, 32)
	spec, err := f.BuildLaunch(context.Background(), testProfile(),
		engine.Runtime{UserDataDir: "/tmp/p1", RelayPort: 4567, CookieKey: key})
	if err != nil {
		t.Fatal(err)
	}
	argv := strings.Join(spec.Args, " ")
	for _, bad := range []string{"--use-mock-keychain", "--remote-allow-origins", "--remote-debugging-port", "schema_version", "abab"} {
		if strings.Contains(strings.ToLower(argv), bad) {
			t.Errorf("argv contains %q", bad)
		}
	}
	for _, want := range []string{"--fury-fp-fd=3", "--fury-oscrypt-fd=4", "--proxy-server=http://127.0.0.1:4567", "--disable-quic", "--window-name=#7 Kenh 1"} {
		if !strings.Contains(argv, want) {
			t.Errorf("argv lacks %q", want)
		}
	}
	if len(spec.Pipes) != 2 || !bytes.Equal(spec.Pipes[1], key) {
		t.Fatal("config and key must travel on pipes 3 and 4")
	}

	spec, _ = f.BuildLaunch(context.Background(), testProfile(), engine.Runtime{UserDataDir: "/tmp/p1", CDP: true, CookieKey: key})
	argv = strings.Join(spec.Args, " ")
	if !strings.Contains(argv, "--remote-debugging-port=0") || strings.Contains(argv, "--proxy-server") {
		t.Fatalf("direct + diagnostics argv wrong: %s", argv)
	}

	spec, _ = f.BuildLaunch(context.Background(), testProfile(), engine.Runtime{UserDataDir: "/tmp/p1", CDPPipe: true, CookieKey: key})
	argv = strings.Join(spec.Args, " ")
	for _, want := range []string{"--fury-fp-fd=5", "--fury-oscrypt-fd=6", "--remote-debugging-pipe"} {
		if !strings.Contains(argv, want) {
			t.Errorf("pipe argv lacks %q: %s", want, argv)
		}
	}
	if strings.Contains(argv, "--remote-debugging-port") {
		t.Errorf("pipe argv must not open a TCP port: %s", argv)
	}
}

func TestStageWidevineFromFakeChrome(t *testing.T) {
	app := filepath.Join(t.TempDir(), "Google Chrome.app")
	root := filepath.Join(app, "Contents/Frameworks/Google Chrome Framework.framework/Versions/154.0/Libraries/WidevineCdm")
	os.MkdirAll(filepath.Join(root, "_platform_specific/mac_arm64"), 0o755)
	os.WriteFile(filepath.Join(root, "_platform_specific/mac_arm64/libwidevinecdm.dylib"), []byte("cdm"), 0o644)
	os.WriteFile(filepath.Join(root, "manifest.json"), []byte(`{"version":"4.10.2934.0"}`), 0o644)
	ud := t.TempDir()
	if err := stageWidevine([]string{"/nope", app}, ud); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"manifest.json", "_platform_specific/mac_arm64/libwidevinecdm.dylib"} {
		if _, err := os.Stat(filepath.Join(ud, "WidevineCdm/4.10.2934.0", rel)); err != nil {
			t.Error(err)
		}
	}
	if err := stageWidevine([]string{"/nope"}, ud); err != errNoChrome {
		t.Fatalf("no chrome err = %v", err)
	}
}

func TestCleanStale(t *testing.T) {
	dirs := paths.Dirs{Root: t.TempDir()}
	f := New(dirs, "/nonexistent")
	if err := os.MkdirAll(f.dir(Version), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.dir(Version), "keep"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Dir(f.dir(Version))
	staging := filepath.Join(parent, ".staging-0.3.5", "Fury.app")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "x"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	tarball := filepath.Join(parent, "fury-core-"+Version+"-macos-arm64.tar.xz")
	if err := os.WriteFile(tarball, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := f.CleanStale(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(parent, ".staging-0.3.5")); !os.IsNotExist(err) {
		t.Fatal("staging survived")
	}
	if _, err := os.Stat(tarball); !os.IsNotExist(err) {
		t.Fatal("tarball survived")
	}
	if _, err := os.Stat(filepath.Join(f.dir(Version), "keep")); err != nil {
		t.Fatal("installed version was removed")
	}
}

func TestPrepareProfileStaysOutOfChromeSignIn(t *testing.T) {
	f := New(paths.Dirs{Root: t.TempDir()}, configBinForTest(t))
	f.chromeApps = []string{"/nope"} // no Chrome: Widevine is skipped, not an error
	udd := t.TempDir()
	if err := f.PrepareProfile(testProfile(), engine.Runtime{UserDataDir: udd}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(udd, "Default", "Preferences"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Signin map[string]any `json:"signin"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m.Signin["allowed"] != false || m.Signin["allowed_on_next_startup"] != false {
		t.Fatalf("signin prefs = %v", m.Signin)
	}
}
