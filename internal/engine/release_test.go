// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sum(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func rel(version string) Release {
	return Release{Engine: "fury", Version: version, Chromium: "155.0.8059.39", Platform: HostPlatform(),
		URL: "https://example.com/core-" + version + ".tar.xz", SHA256: strings.Repeat("a", 64),
		Files: map[string]string{"App/bin": strings.Repeat("b", 64)}, Bin: "App/bin", Catalogue: "c"}
}

func TestReleaseValidate(t *testing.T) {
	if err := rel("1.0.0").Validate(); err != nil {
		t.Fatal(err)
	}
	bad := map[string]func(r *Release){
		"http":         func(r *Release) { r.URL = "http://example.com/a.zip" },
		"exe":          func(r *Release) { r.URL = "https://example.com/a.exe" },
		"version":      func(r *Release) { r.Version = "1.0; rm" },
		"platform":     func(r *Release) { r.Platform = "plan9-386" },
		"sha":          func(r *Release) { r.SHA256 = "xyz" },
		"no bin hash":  func(r *Release) { r.Bin = "App/other" },
		"escape":       func(r *Release) { r.Files = map[string]string{"../x": strings.Repeat("b", 64)}; r.Bin = "../x" },
		"absolute":     func(r *Release) { r.Files["/etc/x"] = strings.Repeat("b", 64) },
		"backslash":    func(r *Release) { r.App = `App\..\..` },
		"minApp":       func(r *Release) { r.MinApp = "soon" },
		"engine":       func(r *Release) { r.Engine = "Fury!" },
		"file sha bad": func(r *Release) { r.Files["App/lib"] = "nope" },
	}
	for name, mod := range bad {
		r := rel("1.0.0")
		r.Files = map[string]string{"App/bin": strings.Repeat("b", 64)}
		mod(&r)
		if r.Validate() == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestNewerVersion(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{{"0.10.0", "0.9.3", true}, {"0.4.5", "0.4.5", false}, {"0.4.5", "0.4.10", false}, {"1.0", "0.99.99", true},
		{"155.0.8059.39", "155.0.8059.31", true}} {
		if got := NewerVersion(c.a, c.b); got != c.want {
			t.Errorf("NewerVersion(%s, %s) = %v", c.a, c.b, got)
		}
	}
}

func TestCatalog(t *testing.T) {
	builtin := rel("0.4.5")
	builtin.Notes = "builtin"
	other := rel("0.3.5")
	other.Platform = "windows-amd64"
	if HostPlatform() == "windows-amd64" {
		other.Platform = "darwin-arm64"
	}
	c := NewCatalog("0.2.0", builtin, rel("0.3.5"), other)
	listedSame := rel("0.4.5")
	listedSame.Notes = "listed"
	tooNew := rel("0.9.0")
	tooNew.MinApp = "0.3.0"
	broken := rel("0.6.0")
	broken.URL = "http://evil/x.zip"
	if err := c.SetListed([]Release{rel("0.5.0"), listedSame, tooNew, broken}); err == nil {
		t.Fatal("the broken release was not reported")
	}
	var got []string
	for _, r := range c.For("fury") {
		got = append(got, r.Version)
	}
	// Newest first; another platform, a too-new MinApp and the broken one are out.
	if strings.Join(got, ",") != "0.5.0,0.4.5,0.3.5" {
		t.Fatalf("For = %v", got)
	}
	if r, _ := c.Find("fury", "0.4.5"); r.Notes != "builtin" {
		t.Fatal("a listed release replaced the compiled-in one")
	}
	if _, ok := c.Find("apostate", "0.4.5"); ok {
		t.Fatal("found under another engine")
	}
	c.SetListed(nil)
	if len(c.For("fury")) != 2 {
		t.Fatal("an empty list does not drop the listed releases")
	}
}

func TestInstallReleaseAndCheck(t *testing.T) {
	archive := makeTarXZ(t, func(tw *tar.Writer) {
		file(tw, "App/bin", "binary", 0o755)
		file(tw, "App/lib", "library", 0o644)
	})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(archive) }))
	defer srv.Close()
	old := http.DefaultClient
	http.DefaultClient = srv.Client()
	defer func() { http.DefaultClient = old }()

	r := rel("1.0.0")
	r.URL, r.SHA256 = srv.URL+"/core.tar.xz", sum(archive)
	r.Files = map[string]string{"App/bin": sum([]byte("binary")), "App/lib": sum([]byte("library"))}
	dir := filepath.Join(t.TempDir(), "fury", "1.0.0")
	ctx := context.Background()

	// A check that fails keeps everything out of place.
	if err := InstallRelease(ctx, r, dir, nil, func(string) error { return os.ErrPermission }); err == nil {
		t.Fatal("a failed check installed")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("version folder exists after a failed check")
	}
	wrong := r
	wrong.Files = map[string]string{"App/bin": sum([]byte("binary")), "App/lib": strings.Repeat("0", 64)}
	if err := InstallRelease(ctx, wrong, dir, nil, nil); err == nil {
		t.Fatal("a file with the wrong hash installed")
	}
	if err := InstallRelease(ctx, r, dir, nil, nil); err != nil {
		t.Fatal(err)
	}
	if st := CheckInstalled(r, dir, nil); !st.Installed || st.Version.Version != "1.0.0" {
		t.Fatalf("status = %+v", st)
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(dir), ".staging-*")); len(left) != 0 {
		t.Fatalf("staging left: %v", left)
	}
	// Any file changed is caught, not only the binary. Changed later: Windows
	// file times move in ~15 ms steps, and an unchanged size and time is what
	// lets a launch skip the hash.
	lib := filepath.Join(dir, "App", "lib")
	os.WriteFile(lib, []byte("librarY"), 0o644)
	later := time.Now().Add(time.Minute)
	os.Chtimes(lib, later, later)
	if st := CheckInstalled(r, dir, nil); st.Installed || st.Problem == "" {
		t.Fatalf("changed library reported as %+v", st)
	}
	os.RemoveAll(dir)
	if st := CheckInstalled(r, dir, nil); st.Installed || st.Problem != "" {
		t.Fatalf("missing install reported as %+v", st)
	}
}
