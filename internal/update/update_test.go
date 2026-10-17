// SPDX-License-Identifier: AGPL-3.0-or-later

package update

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"0.2.0", "0.1.0", true}, {"0.10.0", "0.9.9", true}, {"1.0.0", "0.99.99", true}, {"v0.2.0", "0.1.0", true},
		{"0.1.0", "0.1.0", false}, {"0.1.0", "0.2.0", false}, {"0.2", "0.1.0", false}, {"x.y.z", "0.1.0", false},
		{"0.2.0-beta", "0.1.0", true}, {"0.1.0", "garbage", false},
	} {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

// release serves a signed manifest and its file from a TLS test server.
type release struct {
	srv  *httptest.Server
	body []byte
	m    Manifest
	key  ed25519.PrivateKey
}

func newRelease(t *testing.T, version string) *release {
	pub, key, _ := ed25519.GenerateKey(nil)
	old := publicKey
	publicKey = pub
	t.Cleanup(func() { publicKey = old })
	r := &release{body: []byte("new AnyProfile build"), key: key}
	r.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/latest.json":
			json.NewEncoder(w).Encode(r.m)
		case "/AnyProfile.zip":
			w.Write(r.body)
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(r.srv.Close)
	sum := sha256.Sum256(r.body)
	r.m = Manifest{Version: version, Notes: "Faster.", Assets: map[string]Asset{}}
	r.sign(Platform(), version, hex.EncodeToString(sum[:]))
	t.Setenv(EnvManifestURL, r.srv.URL+"/latest.json")
	return r
}

func (r *release) sign(platform, version, sum string) {
	r.m.Assets[platform] = Asset{URL: r.srv.URL + "/AnyProfile.zip", SHA256: sum,
		Sig: base64.StdEncoding.EncodeToString(ed25519.Sign(r.key, SignedMessage(version, platform, sum)))}
}

func TestFetchVerifyDownload(t *testing.T) {
	r := newRelease(t, "9.0.0")
	ctx := context.Background()
	m, err := Fetch(ctx, r.srv.Client())
	if err != nil || m.Version != "9.0.0" || m.Notes != "Faster." {
		t.Fatalf("fetch = %+v, %v", m, err)
	}
	a, err := m.AssetFor(Platform())
	if err != nil {
		t.Fatal(err)
	}
	var seen int64
	p, err := Download(ctx, r.srv.Client(), a, t.TempDir(), func(done, _ int64) { seen = done })
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != string(r.body) || seen != int64(len(r.body)) || filepath.Base(p) != "AnyProfile.zip" {
		t.Fatalf("downloaded %q to %s, progress %d", b, p, seen)
	}
}

func TestRefusesWhatTheKeyDidNotSign(t *testing.T) {
	r := newRelease(t, "9.0.0")
	good := r.m.Assets[Platform()]

	// The file on the server changed after signing: removed after download.
	r.body = []byte("tampered build")
	dir := t.TempDir()
	if _, err := Download(context.Background(), r.srv.Client(), good, dir, nil); err == nil {
		t.Fatal("tampered file accepted")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("tampered file left behind: %v", entries)
	}

	// An older signed release relabelled as newer: the signature names its version.
	m := r.m
	m.Version = "10.0.0"
	if _, err := m.AssetFor(Platform()); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("relabelled version: %v", err)
	}
	// Another platform's file offered for this one.
	m = Manifest{Version: "9.0.0", Assets: map[string]Asset{Platform(): r.m.Assets[Platform()]}}
	r.sign("plan9-mips", "9.0.0", good.SHA256)
	m.Assets[Platform()] = r.m.Assets["plan9-mips"]
	if _, err := m.AssetFor(Platform()); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("other platform's signature: %v", err)
	}
	// Signed by someone else.
	_, other, _ := ed25519.GenerateKey(nil)
	r.key = other
	r.sign(Platform(), "9.0.0", good.SHA256)
	if _, err := r.m.AssetFor(Platform()); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("foreign key: %v", err)
	}
	// No file for this platform, or plain http.
	if _, err := (Manifest{Version: "9.0.0"}).AssetFor(Platform()); !errors.Is(err, ErrNoAsset) {
		t.Fatalf("missing asset: %v", err)
	}
	plain := Manifest{Version: "9.0.0", Assets: map[string]Asset{Platform(): {URL: "http://x/y", SHA256: good.SHA256, Sig: good.Sig}}}
	if _, err := plain.AssetFor(Platform()); err == nil {
		t.Fatal("http download accepted")
	}
}

func TestNoBuiltInKeyFailsClosed(t *testing.T) {
	r := newRelease(t, "9.0.0")
	publicKey = nil
	if _, err := r.m.AssetFor(Platform()); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchRejectsBadManifest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(`{"version": "latest"}`))
	}))
	defer srv.Close()
	t.Setenv(EnvManifestURL, srv.URL)
	if _, err := Fetch(context.Background(), srv.Client()); err == nil {
		t.Fatal("bad version accepted")
	}
}

// Current must match wails.json, which make build passes in; the default is
// what `go run` and tests see.
func TestCurrentMatchesWailsJSON(t *testing.T) {
	b, err := os.ReadFile("../../wails.json")
	if err != nil {
		t.Fatal(err)
	}
	var w struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(b, &w); err != nil || w.Info.ProductVersion != Current {
		t.Fatalf("wails.json %q, Current %q, %v", w.Info.ProductVersion, Current, err)
	}
}

func TestEngineListSignature(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(nil)
	compact := []byte(`[{"engine":"fury","version":"0.4.6"}]`)
	msg, _ := EnginesMessage(compact)
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(key, msg))

	// latest.json is indented after signing: the list still verifies.
	m := Manifest{Version: "0.2.0", Engines: compact, EnginesSig: sig}
	pretty, _ := json.MarshalIndent(m, "", "  ")
	var back Manifest
	if err := json.Unmarshal(pretty, &back); err != nil {
		t.Fatal(err)
	}
	rs, err := verifyEngines(pub, back.Engines, back.EnginesSig)
	if err != nil || len(rs) != 1 || rs[0].Version != "0.4.6" {
		t.Fatalf("verify = %+v, %v", rs, err)
	}
	// One changed value, or another key: refused.
	if _, err := verifyEngines(pub, []byte(`[{"engine":"fury","version":"0.4.7"}]`), sig); !errors.Is(err, ErrBadEnginesSignature) {
		t.Fatalf("changed list = %v", err)
	}
	other, _, _ := ed25519.GenerateKey(nil)
	if _, err := verifyEngines(other, compact, sig); !errors.Is(err, ErrBadEnginesSignature) {
		t.Fatalf("other key = %v", err)
	}
	if _, err := verifyEngines(pub, compact, "not base64!"); err == nil {
		t.Fatal("a bad signature passed")
	}
}
