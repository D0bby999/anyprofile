// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package avo

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
