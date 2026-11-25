// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin && avointegration

// Run live end-to-end Cuttlefish Metal GPU + GMS + vnet proxy test:
//
//	go test -tags avointegration -run TestAvoLive -v -timeout 15m ./internal/mobile/avo/
package avo

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/vm"
	"github.com/D0bby999/anyprofile/internal/netproxy"
)

type connectProxy struct {
	mu      sync.Mutex
	targets []string
}

func (p *connectProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	p.targets = append(p.targets, r.Host)
	p.mu.Unlock()
	if r.Method != http.MethodConnect {
		http.Error(w, "connect only", http.StatusBadGateway)
		return
	}
	out, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	hj, _ := w.(http.Hijacker)
	in, buf, err := hj.Hijack()
	if err != nil {
		out.Close()
		return
	}
	_, _ = io.WriteString(in, "HTTP/1.1 200 Connection established\r\n\r\n")
	go func() {
		_, _ = io.Copy(out, buf)
		_ = out.Close()
	}()
	_, _ = io.Copy(in, out)
	_ = in.Close()
}

func (p *connectProxy) saw(sub string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, t := range p.targets {
		if strings.Contains(t, sub) {
			return true
		}
	}
	return false
}

func TestAvoLiveMetalGMSProxy(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	qemuRuntime := filepath.Join(home, "Library", "Application Support", "AnyProfile", "mobile", "runtime")
	if !Staged(RootFor(qemuRuntime)) {
		t.Skip("avo runtime not staged")
	}

	devicesDir := t.TempDir()
	drv := New(qemuRuntime, devicesDir, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	id := vm.IdentityFor("live01", "s24ultra")
	dev := mobile.Device{
		ID:           "live01",
		Name:         "S24 Ultra 120Hz",
		Driver:       "avo",
		RAMMB:        3072,
		CPUs:         6,
		Preset:       "interactive",
		ScreenW:      780,
		ScreenH:      1688,
		Density:      320,
		WithGMS:      true,
		IdentityJSON: vm.MarshalIdentity(id),
	}

	ref, ver, err := drv.Create(ctx, dev)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Logf("created ref=%s version=%s", ref, ver)

	px := &connectProxy{}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = http.Serve(ln, px) }()
	defer ln.Close()
	up := &netproxy.Upstream{Scheme: "http", Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port}

	run, err := drv.Start(ctx, dev, mobile.StartOptions{
		Upstream: up,
		Timezone: "Asia/Ho_Chi_Minh",
		Locale:   "vi-VN",
		Country:  "VN",
		Headless: false,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		sctx, scancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer scancel()
		_ = drv.Stop(sctx, dev)
	}()
	t.Logf("started pid=%d adbPort=%d", run.Launch.ADBPort, run.Launch.ADBPort)

	a, ok := drv.ADB(dev)
	if !ok {
		t.Fatal("ADB client missing")
	}
	sh := func(cmd string) string {
		c, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		out, err := a.Shell(c, cmd)
		if err != nil {
			t.Logf("shell %q err=%v out=%q", cmd, err, out)
		}
		return strings.TrimSpace(out)
	}

	if got := sh("getprop sys.boot_completed"); got != "1" {
		t.Fatalf("sys.boot_completed = %q", got)
	}
	gles := sh("dumpsys SurfaceFlinger | grep GLES:")
	t.Logf("SurfaceFlinger %s", gles)
	if !strings.Contains(gles, "GFXStream") {
		t.Fatalf("expected Virtio-GPU GFXStream in SurfaceFlinger GLES, got %q", gles)
	}

	model := sh("getprop ro.product.model")
	hw := sh("getprop ro.hardware")
	t.Logf("model=%q hardware=%q", model, hw)
	if model != "SM-S928B" {
		t.Fatalf("model = %q, want SM-S928B", model)
	}
	if hw != "qcom" {
		t.Fatalf("ro.hardware = %q, want qcom", hw)
	}

	vending := sh("pm path com.android.vending")
	gms := sh("pm path com.google.android.gms")
	t.Logf("vending=%q gms=%q", vending, gms)
	if !strings.Contains(vending, ".apk") || !strings.Contains(gms, ".apk") {
		t.Fatalf("Play Store or GMS missing: vending=%q gms=%q", vending, gms)
	}

	// Verify outbound TCP/TLS goes through vnet -> local HTTP CONNECT proxy.
	// Android 16 Cuttlefish includes boringssl / toybox / am; curl or nc works.
	netCheck := sh("toybox nc -w 5 example.com 80 </dev/null; ip route get 1.1.1.1")
	t.Logf("netCheck=%q proxyTargets=%v", netCheck, px.targets)
	if !px.saw("example.com") && !px.saw(":443") && !px.saw(":80") {
		t.Fatalf("traffic did not hit vnet upstream proxy; targets=%v", px.targets)
	}

	if err := drv.Pause(dev.ID); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if !drv.Paused(dev.ID) {
		t.Fatal("expected Paused() == true")
	}
	if err := drv.Resume(dev.ID); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if drv.Paused(dev.ID) {
		t.Fatal("expected Paused() == false after Resume")
	}
	if !drv.Windowed(dev.ID) {
		t.Fatal("expected anyprofile-view window open")
	}

	if glRend := sh("getprop debug.avo.gl.renderer"); glRend != "Adreno (TM) 750" {
		t.Fatalf("debug.avo.gl.renderer = %q, want Adreno (TM) 750", glRend)
	}
	if glVer := sh("getprop ro.opengles.version"); glVer != "196610" {
		t.Fatalf("ro.opengles.version = %q, want 196610 (OpenGL ES 3.2)", glVer)
	}
	if !strings.Contains(gles, "OpenGL ES 3.2") {
		t.Fatalf("expected OpenGL ES 3.2 in SurfaceFlinger GLES, got %q", gles)
	}
	if layerCheck := sh("ls /data/local/debug/vulkan/libVkLayer_avo_gfxstream_sync.so /data/local/debug/gles/libGLES_avo_profile.so"); strings.Contains(layerCheck, "No such") {
		t.Fatalf("GPU debug layers missing: %q", layerCheck)
	}
	if err := drv.SetWindowMuted(ctx, dev, true); err != nil {
		t.Fatalf("SetWindowMuted(true): %v", err)
	}
	if !drv.WindowMuted(dev) {
		t.Fatal("expected WindowMuted == true")
	}
	_ = drv.SetWindowMuted(ctx, dev, false)

	// Verify Adopt rebuilds vnet over the live net.sock after dropping the previous vnet.
	if p := drv.proc(dev.ID); p != nil && p.net != nil {
		_ = p.net.Close()
		_ = p.bridge.Close()
	}
	drv2 := New(qemuRuntime, devicesDir, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if _, err := drv2.Adopt(ctx, dev, run.Launch, up); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)
	px.mu.Lock()
	before := len(px.targets)
	px.mu.Unlock()
	a2, ok := drv2.ADB(dev)
	if !ok {
		t.Fatal("adopted ADB missing")
	}
	if out, err := a2.Shell(ctx, "toybox nc -w 5 example.com 80 </dev/null"); err != nil {
		t.Logf("adopted shell err=%v out=%q", err, out)
	}
	px.mu.Lock()
	after := len(px.targets)
	px.mu.Unlock()
	if after <= before {
		t.Fatalf("adopted vnet did not forward traffic (before=%d after=%d)", before, after)
	}
	if p2 := drv2.proc(dev.ID); p2 != nil {
		_ = drv2.Stop(ctx, dev)
	}
}

func TestAvoLiveCameras(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	qemuRuntime := filepath.Join(home, "Library", "Application Support", "AnyProfile", "mobile", "runtime")
	if !Staged(RootFor(qemuRuntime)) {
		t.Skip("avo runtime not staged")
	}

	devicesDir := t.TempDir()
	drv := New(qemuRuntime, devicesDir, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	dev := mobile.Device{
		ID:      "camtest01",
		Name:    "Camera Test",
		Driver:  "avo",
		RAMMB:   3072,
		CPUs:    4,
		ScreenW: 780,
		ScreenH: 1688,
		Density: 320,
		WithGMS: false,
	}
	if _, _, err := drv.Create(ctx, dev); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := drv.Start(ctx, dev, mobile.StartOptions{Headless: true}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = drv.Stop(context.Background(), dev) }()

	a, ok := drv.ADB(dev)
	if !ok {
		t.Fatal("ADB missing")
	}
	videos, err := a.Shell(ctx, "ls -la /dev/video*")
	if err != nil {
		t.Fatalf("ls /dev/video*: %v (%s)", err, videos)
	}
	t.Logf("video nodes:\n%s", videos)
	if !strings.Contains(videos, "/dev/video1") || !strings.Contains(videos, "/dev/video2") {
		t.Fatalf("expected /dev/video1 and /dev/video2, got:\n%s", videos)
	}
	camDump, err := a.Shell(ctx, "dumpsys media.camera | grep -E 'Number of camera devices|Device [0-9]|Facing|v4l2|external' | head -n 30")
	if err != nil {
		t.Fatalf("dumpsys media.camera: %v", err)
	}
	t.Logf("dumpsys media.camera:\n%s", camDump)
	if !strings.Contains(camDump, "Number of camera devices: 2") {
		t.Fatalf("expected 2 camera devices in dumpsys media.camera, got:\n%s", camDump)
	}
}
