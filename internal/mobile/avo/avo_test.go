// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package avo

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/vm"
)

func TestLocateRuntime(t *testing.T) {
	dir := t.TempDir()
	for _, rel := range []string{
		"bin/anyprofile-vmm",
		"bin/anyprofile-view",
		"bin/adb",
		"gfxstream/lib64/libgfxstream_backend.dylib",
		"image/123/boot.img",
		"image/123/vendor_boot.img",
		"image/123/super.img",
		"image/123/userdata.img",
	} {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "image/current"), []byte("123\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := Locate(dir)
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if rt.ImageVersion != "123" {
		t.Fatalf("ImageVersion = %q, want 123", rt.ImageVersion)
	}
	t.Setenv("ANYPROFILE_AVO_DIR", dir)
	if got := RootFor("/ignored/qemu"); got != dir {
		t.Fatalf("RootFor env = %q, want %q", got, dir)
	}
}

func TestRouterUpdatesExistingQEMUAndAddsMetal(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ANYPROFILE_AVO_DIR", filepath.Join(root, "avo"))
	qemu, devices := filepath.Join(root, "runtime"), filepath.Join(root, "devices")
	legacy := vm.New(qemu, devices, nil)
	metal := New(qemu, devices, nil)
	r := NewRouter(metal, legacy)
	if got := r.UpdateInstallers(); len(got) != 0 {
		t.Fatalf("fresh install scheduled: %v", got)
	}
	for _, rel := range []string{"bin/qemu-system-aarch64", "bin/qemu-img", "bin/adb", "bin/scrcpy", "share/edk2-aarch64-code.fd", "share/edk2-arm-vars.fd", "share/scrcpy-server", "VERSION"} {
		p := filepath.Join(qemu, "qemu", rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("1.0.1"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if got := r.UpdateInstallers(); len(got) != 2 || got[0] != legacy || got[1] != metal {
		t.Fatalf("QEMU and Metal should update separately: %v", got)
	}
	if got := r.forDevice(mobile.Device{Driver: "vm"}); got != legacy {
		t.Fatal("legacy device lost its driver")
	}
}

func TestADBBridgeAndControl(t *testing.T) {
	dir := t.TempDir()
	vsock := filepath.Join(dir, "v.sock")
	ln, err := net.Listen("unix", vsock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				r := bufio.NewReader(c)
				line, err := r.ReadString('\n')
				if err != nil || line != "CONNECT 5555\n" {
					return
				}
				_, _ = io.WriteString(c, "OK 10001\n")
				_, _ = io.Copy(c, r)
			}(c)
		}
	}()

	b, err := startADBBridge(vsock)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", b.Port), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = io.WriteString(conn, "ping-adb")
	buf := make([]byte, 8)
	if _, err := io.ReadFull(conn, buf); err != nil || string(buf) != "ping-adb" {
		t.Fatalf("bridge echo = %q, %v", buf, err)
	}

	ctrl := filepath.Join(dir, "c.sock")
	cln, err := net.Listen("unix", ctrl)
	if err != nil {
		t.Fatal(err)
	}
	defer cln.Close()
	go func() {
		for {
			c, err := cln.Accept()
			if err != nil {
				return
			}
			line, _ := bufio.NewReader(c).ReadString('\n')
			if strings.HasPrefix(line, "bad") {
				_, _ = io.WriteString(c, "error denied\n")
			} else {
				_, _ = io.WriteString(c, "ok\n")
			}
			c.Close()
		}
	}()
	if _, err := sendControl(context.Background(), ctrl, "pause"); err != nil {
		t.Fatalf("sendControl ok: %v", err)
	}
	if _, err := sendControl(context.Background(), ctrl, "bad"); err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("sendControl bad: got %v", err)
	}
}

func TestBuildLaunchSpecAndClusters(t *testing.T) {
	dir := t.TempDir()
	profiles := filepath.Join(dir, "profiles")
	layers := filepath.Join(dir, "layers")
	if err := os.MkdirAll(profiles, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(layers, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"libVkLayer_avo_gfxstream_sync.so", "libVkLayer_avo_profile.so", "libGLES_avo_profile.so"} {
		if err := os.WriteFile(filepath.Join(layers, f), []byte("so"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tpl := `{
  "schema": 1,
  "id": "samsung-galaxy-s24-ultra",
  "name": "Samsung Galaxy S24 Ultra",
  "properties": {
    "ro.product.brand": "samsung",
    "ro.product.model": "SM-S928B"
  },
  "hardware": {
    "soc": {
      "manufacturer": "QTI",
      "platform": "pineapple",
      "properties": {"ro.vendor.qti.soc_model": "SM8650"}
    },
    "cpu": {
      "clusters": [
        {"cores": 2, "implementer": 65, "part": 3456, "maxFreqKHz": 2265600, "minFreqKHz": 300000},
        {"cores": 1, "implementer": 65, "part": 3458, "maxFreqKHz": 3302400, "minFreqKHz": 576000}
      ]
    },
    "gpu": {
      "glVendor": "Qualcomm",
      "glRenderer": "Adreno (TM) 750"
    }
  }
}`
	if err := os.WriteFile(filepath.Join(profiles, "samsung-galaxy-s24-ultra.json"), []byte(tpl), 0o644); err != nil {
		t.Fatal(err)
	}

	rt := Runtime{ProfilesDir: profiles, LayersDir: layers}
	id := vm.IdentityFor("dev1", "s24ultra")
	dev := mobile.Device{
		ID:           "dev1",
		CPUs:         4,
		IdentityJSON: vm.MarshalIdentity(id),
	}
	spec := BuildLaunchSpec(rt, dev, true)
	if spec.Hardware != "qcom" {
		t.Fatalf("Hardware = %q, want qcom", spec.Hardware)
	}
	for _, want := range []string{
		"ro.product.model=SM-S928B",
		"ro.vendor.qti.soc_model=SM8650",
		"debug.avo.gl.vendor=Qualcomm",
		"debug.avo.gl.renderer=Adreno (TM) 750",
		"debug.vulkan.layer.9=VK_LAYER_AVO_gfxstream_sync",
		"debug.avo.gl.es32=1",
		"ro.opengles.version=196610",
		"debug.gles.layers=libGLES_avo_profile.so",
	} {
		if !strings.Contains(spec.PropertiesText, want) {
			t.Fatalf("PropertiesText missing %q:\n%s", want, spec.PropertiesText)
		}
	}
	// 4 vCPUs fitted across 2+1 clusters grows the slowest cluster to 3 cores + 1 prime core.
	if spec.CPUClusters != "3x410fd800@300000-2265600,1x410fd820@576000-3302400" {
		t.Fatalf("CPUClusters = %q", spec.CPUClusters)
	}
}

func makeZipBytes(t *testing.T, files map[string]string) ([]byte, string, map[string]string) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	hashes := make(map[string]string, len(files))
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, content); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte(content))
		hashes[name] = hex.EncodeToString(sum[:])
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	raw := buf.Bytes()
	sum := sha256.Sum256(raw)
	return raw, hex.EncodeToString(sum[:]), hashes
}

func TestInstallAvoRuntimeAndCuttlefish(t *testing.T) {
	rtZip, rtSHA, rtFiles := makeZipBytes(t, map[string]string{
		"bin/anyprofile-vmm":  "vmm-bin",
		"bin/anyprofile-view": "view-bin",
		"bin/adb":             "adb-bin",
		"gfxstream/lib64/libgfxstream_backend.dylib": "gfx-lib",
	})
	imgZip, imgSHA, _ := makeZipBytes(t, map[string]string{
		"boot.img":        "boot",
		"vendor_boot.img": "vboot",
		"super.img":       "super",
		"userdata.img":    "user",
	})

	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/anymobile-avo-1.0.0.zip":
			_, _ = w.Write(rtZip)
		case strings.HasSuffix(r.URL.Path, "/url"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"signedUrl":"%s/cuttlefish.zip"}`, srv.URL)
		case r.URL.Path == "/cuttlefish.zip":
			_, _ = w.Write(imgZip)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	oldClient := http.DefaultClient
	http.DefaultClient = srv.Client()
	defer func() { http.DefaultClient = oldClient }()

	prevAPI, prevSHA := cuttlefishAPIBase, cuttlefishSHA256
	cuttlefishAPIBase, cuttlefishSHA256 = srv.URL, imgSHA
	defer func() { cuttlefishAPIBase, cuttlefishSHA256 = prevAPI, prevSHA }()

	avoRoot := t.TempDir()
	t.Setenv("ANYPROFILE_AVO_DIR", avoRoot)
	drv := New(filepath.Join(t.TempDir(), "runtime"), t.TempDir(), nil)

	if err := drv.OfferRuntime([]engine.Release{{
		Engine:    RuntimeEngine,
		Version:   "1.0.0",
		Chromium:  "16.0",
		Platform:  engine.HostPlatform(),
		URL:       srv.URL + "/anymobile-avo-1.0.0.zip",
		SHA256:    rtSHA,
		Files:     rtFiles,
		Bin:       "bin/anyprofile-vmm",
		Catalogue: RuntimeEngine,
	}}); err != nil {
		t.Fatalf("OfferRuntime: %v", err)
	}
	if !drv.HasOffer() {
		t.Fatal("expected HasOffer == true")
	}
	st0 := drv.InstallStatus()
	if st0.Runtime.Installed != "" || st0.Runtime.Latest != "1.0.0" {
		t.Fatalf("initial InstallStatus = %+v", st0)
	}
	if err := drv.Install(context.Background(), nil); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := drv.Available(); err != nil {
		t.Fatalf("Available after Install: %v", err)
	}
	st1 := drv.InstallStatus()
	if st1.Runtime.Installed != "1.0.0" || st1.Image.Installed != CuttlefishBuild {
		t.Fatalf("post-install InstallStatus = %+v", st1)
	}
}
