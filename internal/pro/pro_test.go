// SPDX-License-Identifier: AGPL-3.0-or-later

package pro

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
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

	"github.com/D0bby999/anyprofile/internal/apperr"
)

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sign(t *testing.T, priv ed25519.PrivateKey, version string, zipBytes []byte) Release {
	sum := sha256.Sum256(zipBytes)
	h := hex.EncodeToString(sum[:])
	return Release{Version: version, SHA256: h, Size: int64(len(zipBytes)),
		Sig: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, SignedMessage(version, h)))}
}

func testKey(t *testing.T) ed25519.PrivateKey {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	old := publicKey
	publicKey = pub
	t.Cleanup(func() { publicKey = old })
	return priv
}

func code(err error) string {
	var e *apperr.Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

func TestSignedMessageMatchesThePackTool(t *testing.T) {
	if string(SignedMessage("1.0.0", "ab")) != "anyprofile-pro\n1.0.0\nab" {
		t.Fatal("must match anyprofile-pro/tools/pack")
	}
	if publicKey == nil {
		t.Fatal("no Pro public key built in")
	}
}

func TestDownloadVerifiesAndInstalls(t *testing.T) {
	priv := testKey(t)
	pack := makeZip(t, map[string]string{"VERSION": "1.2.0\n", "recipes/a.md": "---\nname: a\n---\nhi\n", "scripts/run.mjs": "x"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(pack) }))
	defer srv.Close()
	c := Client{Base: srv.URL, HTTP: srv.Client()}
	root := filepath.Join(t.TempDir(), "pro")

	rel := sign(t, priv, "1.2.0", pack)
	rel.URL = srv.URL + "/v1/download/1.2.0"
	zp, err := c.Download(context.Background(), rel, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := Install(zp, "1.2.0", root); err != nil {
		t.Fatal(err)
	}
	if Installed(root) != "1.2.0" {
		t.Fatalf("installed %q", Installed(root))
	}
	if _, err := os.Stat(filepath.Join(Recipes(root), "a.md")); err != nil {
		t.Fatal(err)
	}

	bad := rel
	bad.Sig = base64.StdEncoding.EncodeToString(make([]byte, 64))
	if _, err := c.Download(context.Background(), bad, t.TempDir()); code(err) != "pro_bad_signature" {
		t.Fatalf("a bad signature gave %v", err)
	}
	other := sign(t, priv, "1.2.0", append([]byte{}, pack[:len(pack)-1]...)) // signed, but not these bytes
	other.URL = rel.URL
	dir := t.TempDir()
	if _, err := c.Download(context.Background(), other, dir); code(err) != "pro_bad_signature" {
		t.Fatalf("bytes that differ from the signed ones gave %v", err)
	}
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Fatal("a failed download left a file behind")
	}
}

func TestInstallSwapsAndKeepsNodeModules(t *testing.T) {
	root := filepath.Join(t.TempDir(), "pro")
	write := func(v string) string {
		p := filepath.Join(t.TempDir(), "p.zip")
		os.WriteFile(p, makeZip(t, map[string]string{"VERSION": v, "scripts/run.mjs": v}), 0o600)
		return p
	}
	if err := Install(write("1.0.0"), "1.0.0", root); err != nil {
		t.Fatal(err)
	}
	nm := filepath.Join(Scripts(root), "node_modules", "dep")
	os.MkdirAll(nm, 0o700)
	if err := Install(write("1.1.0"), "1.1.0", root); err != nil {
		t.Fatal(err)
	}
	if Installed(root) != "1.1.0" || !dirExists(nm) {
		t.Fatal("update lost the version or node_modules")
	}
	if readVersion(previous(root)) != "1.0.0" {
		t.Fatal("the previous pack is not kept")
	}
	// A pack that says another version is refused, and the installed one stays.
	if err := Install(write("9.9.9"), "1.2.0", root); err == nil || Installed(root) != "1.1.0" {
		t.Fatalf("a mislabelled pack: err %v, installed %s", err, Installed(root))
	}
}

func TestUnzipRefusesEscapes(t *testing.T) {
	for _, name := range []string{"../evil", "/abs", "a/../../evil", `a\..\evil`} {
		p := filepath.Join(t.TempDir(), "p.zip")
		os.WriteFile(p, makeZip(t, map[string]string{"VERSION": "1.0.0", name: "x"}), 0o600)
		if err := Install(p, "1.0.0", filepath.Join(t.TempDir(), "pro")); code(err) != "pro_bad_signature" {
			t.Errorf("%q: %v", name, err)
		}
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	h := &zip.FileHeader{Name: "link"}
	h.SetMode(os.ModeSymlink | 0o777)
	w, _ := zw.CreateHeader(h)
	w.Write([]byte("/etc/passwd"))
	zw.Close()
	p := filepath.Join(t.TempDir(), "p.zip")
	os.WriteFile(p, buf.Bytes(), 0o600)
	if err := Install(p, "1.0.0", filepath.Join(t.TempDir(), "pro")); code(err) != "pro_bad_signature" {
		t.Errorf("symlink: %v", err)
	}
}

func TestServerErrorsBecomeCodes(t *testing.T) {
	answers := map[string]int{"bad_key": 401, "revoked": 401, "too_many_devices": 409, "not_yet": 404}
	for e, status := range answers {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"error": e})
		}))
		c := Client{Base: srv.URL, HTTP: srv.Client()}
		_, err := c.Check(context.Background(), Ask{Key: "k", InstallID: "i"})
		_, cerr := c.Claim(context.Background(), "r")
		srv.Close()
		want := map[string]string{"bad_key": "pro_bad_key", "revoked": "pro_revoked", "too_many_devices": "pro_too_many_devices"}[e]
		if e == "not_yet" {
			if !errors.Is(cerr, ErrNotYet) {
				t.Errorf("not_yet: %v", cerr)
			}
			continue
		}
		if code(err) != want {
			t.Errorf("%s: %v", e, err)
		}
	}
	c := Client{Base: "http://127.0.0.1:1", HTTP: http.DefaultClient}
	if _, err := c.Check(context.Background(), Ask{}); code(err) != "pro_offline" {
		t.Errorf("offline: %v", err)
	}
}
