// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package vm

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/D0bby999/anyprofile/internal/engine"
)

func hexSum(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func zipOf(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, b := range files {
		f, _ := w.Create(name)
		f.Write(b)
	}
	w.Close()
	return buf.Bytes()
}

func TestImageBuiltinIsValid(t *testing.T) {
	for _, r := range ImageBuiltin() {
		if err := r.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}

// Install fetches the runtime and the image from the catalog and lays them
// out where Locate finds them.
func TestInstall(t *testing.T) {
	runtimeFiles := map[string][]byte{}
	for _, f := range []string{"bin/qemu-system-aarch64", "bin/qemu-img", "share/edk2-aarch64-code.fd",
		"share/edk2-arm-vars.fd", "bin/adb", "bin/scrcpy", "share/scrcpy-server"} {
		runtimeFiles[f] = []byte("x-" + f)
	}
	imageFiles := map[string][]byte{utmData + "/vda.qcow2": []byte("system"), utmData + "/vdb.qcow2": []byte("data")}
	archives := map[string][]byte{"/runtime.zip": zipOf(t, runtimeFiles), "/image.zip": zipOf(t, imageFiles)}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(archives[r.URL.Path]) }))
	defer srv.Close()
	old := http.DefaultClient
	http.DefaultClient = srv.Client()
	defer func() { http.DefaultClient = old }()

	sums := func(m map[string][]byte) map[string]string {
		out := map[string]string{}
		for k, v := range m {
			out[k] = hexSum(v)
		}
		return out
	}
	cat := engine.NewCatalog("",
		engine.Release{Engine: RuntimeEngine, Version: "0.1.0", Chromium: "11.1.2", Platform: engine.HostPlatform(),
			URL: srv.URL + "/runtime.zip", SHA256: hexSum(archives["/runtime.zip"]), Files: sums(runtimeFiles),
			Bin: "bin/qemu-system-aarch64", Catalogue: RuntimeEngine},
		engine.Release{Engine: ImageEngine, Version: "23.2.1", Chromium: "16.0", Platform: engine.HostPlatform(),
			URL: srv.URL + "/image.zip", SHA256: hexSum(archives["/image.zip"]), Files: sums(imageFiles),
			Bin: utmData + "/vda.qcow2", Catalogue: ImageEngine})
	root := t.TempDir()
	d := New(root, t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	d.SetCatalog(cat)
	if st := d.InstallStatus(); st.Runtime.Installed != "" || st.Image.Installed != "" || st.Runtime.Latest != "0.1.0" {
		t.Fatalf("before: %+v", st)
	}
	if d.Available() == nil {
		t.Fatal("available before install")
	}
	seen := map[string]bool{}
	if err := d.Install(context.Background(), func(e string, done, total int64) { seen[e] = true }); err != nil {
		t.Fatal(err)
	}
	if !seen[RuntimeEngine] || !seen[ImageEngine] {
		t.Fatalf("progress for %v", seen)
	}
	st := d.InstallStatus()
	if st.Runtime.Installed != "0.1.0" || st.Image.Installed != "23.2.1" {
		t.Fatalf("after: %+v", st)
	}
	rt, err := Locate(root)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(rt.ImageDir, "vda.qcow2")); string(b) != "system" {
		t.Fatalf("image dir %s", rt.ImageDir)
	}
	// Installed and current: nothing is downloaded again.
	archives["/image.zip"] = nil
	if err := d.Install(context.Background(), func(string, int64, int64) {}); err != nil {
		t.Fatal(err)
	}
}

func TestInstallWithoutRuntimeRelease(t *testing.T) {
	d := New(t.TempDir(), t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	d.SetCatalog(engine.NewCatalog("", ImageBuiltin()...))
	if err := d.Install(context.Background(), func(string, int64, int64) {}); err != errNoRelease {
		t.Fatalf("%v", err)
	}
}
