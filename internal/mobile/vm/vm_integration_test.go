// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin && vmintegration

// Runs a real VM: go test -tags vmintegration -run TestVM -v -timeout 30m ./internal/mobile/vm/
// with ANYPROFILE_VM_RUNTIME set to a runtime laid out by scripts/dev-android-vm-runtime.sh.

package vm

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/adb"
	"github.com/D0bby999/anyprofile/internal/netproxy"
)

// connectProxy is an HTTP CONNECT proxy that records where it was asked to go.
type connectProxy struct {
	mu      sync.Mutex
	targets []string
	refuse  bool
}

func (p *connectProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	p.targets = append(p.targets, r.Host)
	refuse := p.refuse
	p.mu.Unlock()
	if r.Method != http.MethodConnect || refuse {
		http.Error(w, "no", http.StatusBadGateway)
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
	in.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
	go func() { io.Copy(out, buf); out.Close() }()
	io.Copy(in, out)
	in.Close()
}

func (p *connectProxy) saw(prefix string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, t := range p.targets {
		if strings.HasPrefix(t, prefix) {
			return true
		}
	}
	return false
}

func TestVM(t *testing.T) {
	rtDir := os.Getenv("ANYPROFILE_VM_RUNTIME")
	if rtDir == "" {
		t.Skip("ANYPROFILE_VM_RUNTIME not set")
	}
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	devs := os.Getenv("ANYPROFILE_VM_DEVICES") // kept for reading qemu.log after a failure
	if devs == "" {
		devs = t.TempDir()
	}
	drv := New(rtDir, devs, log)
	if err := drv.Available(); err != nil {
		t.Fatal(err)
	}
	d := mobile.Device{ID: "it" + strconv.FormatInt(time.Now().Unix()%1e6, 10), Name: "Integration", RAMMB: 3072, CPUs: 4}

	start := time.Now()
	ref, img, err := drv.Create(ctx, d)
	if err != nil {
		t.Fatal(err)
	}
	d.Ref = ref
	t.Logf("created in %s, image %s", time.Since(start).Round(time.Second), img)
	defer func() {
		if !t.Failed() {
			drv.Delete(ctx, d)
		}
	}()

	px := &connectProxy{}
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	go http.Serve(ln, px)
	defer ln.Close()
	up := &netproxy.Upstream{Scheme: "http", Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port}

	start = time.Now()
	run, err := drv.Start(ctx, d, mobile.StartOptions{Upstream: up, Timezone: "Asia/Tokyo", Locale: "ja-JP"})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("started in %s, adb port %d", time.Since(start).Round(time.Second), run.Launch.ADBPort)
	a := drv.proc(d.ID).adb
	sh := func(line string) string {
		c, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		out, err := a.Shell(c, line)
		if err != nil {
			t.Logf("%s: %v", line, err)
		}
		return out
	}

	if got := sh("getprop sys.boot_completed"); got != "1" {
		t.Fatalf("boot_completed = %q", got)
	}
	if got := sh("settings get global device_provisioned"); got != "1" {
		t.Errorf("device_provisioned = %q", got)
	}
	if got := sh("date +%s"); got != "" {
		n, _ := strconv.ParseInt(got, 10, 64)
		if d := time.Now().Unix() - n; d > 120 || d < -120 {
			t.Errorf("the VM's clock is %ds off: HTTPS fails on certificates newer than the image", d)
		}
	}
	if got := sh("getprop persist.sys.timezone"); got != "Asia/Tokyo" {
		t.Errorf("timezone = %q", got)
	}
	if got := sh("cmd locale get-app-locales " + browserPkg); !strings.Contains(got, "ja-JP") {
		t.Errorf("browser locale = %q", got)
	}
	if os.Getenv("ANYPROFILE_VM_EXPLORE") != "" {
		for _, l := range strings.Split(os.Getenv("ANYPROFILE_VM_EXPLORE"), ";;") {
			if strings.HasPrefix(l, "ADB:") {
				c, cancel := context.WithTimeout(ctx, 60*time.Second)
				out, err := a.Run(c, strings.Fields(l[4:])...)
				cancel()
				t.Logf("EXPLORE %s => %q %v", l, out, err)
				time.Sleep(3 * time.Second)
				a.Connect(ctx)
				continue
			}
			if strings.HasPrefix(l, "WAIT") {
				c, cancel := context.WithTimeout(ctx, 3*time.Minute)
				err := a.WaitBoot(c, nil)
				cancel()
				t.Logf("EXPLORE wait boot: %v", err)
				continue
			}
			t.Logf("EXPLORE %s => %q", l, sh(l+" 2>&1 | head -c 600"))
		}
	}
	t.Logf("ip: %s", sh("ip -4 addr show eth0 | grep inet"))
	t.Logf("ipv6: %q", sh("ip -6 addr show scope global"))

	// TCP leaves through the proxy.
	out := sh(`printf 'GET / HTTP/1.0\r\nHost: one.one.one.one\r\n\r\n' | toybox nc -w 10 1.1.1.1 80 | head -n 1`)
	t.Logf("http via proxy: %q", out)
	if !px.saw("1.1.1.1:80") {
		t.Error("the proxy did not see the VM's TCP connection to 1.1.1.1:80")
	}
	// Names resolve, through the proxy (DNS over HTTPS to 1.1.1.1:443).
	t.Logf("resolve: %q", sh("ping -c 1 -W 2 example.com 2>&1 | head -n 1"))
	if !px.saw("1.1.1.1:443") && !px.saw("1.1.1.1:53") {
		t.Error("DNS did not go through the proxy")
	}
	// UDP has no way out.
	if got := sh(`printf 'x' | timeout 5 toybox nc -u 8.8.8.8 53 | head -c 1 | wc -c`); got != "0" {
		t.Errorf("udp answered: %q", got)
	}

	// The proxy refusing is no network, not the host's.
	px.mu.Lock()
	px.refuse = true
	px.mu.Unlock()
	if got := sh(`printf 'GET / HTTP/1.0\r\n\r\n' | toybox nc -w 5 93.184.215.14 80 | head -c 12`); strings.HasPrefix(got, "HTTP") {
		t.Errorf("got through with the proxy refusing: %q", got)
	}

	px.mu.Lock()
	px.refuse, px.targets = false, nil
	px.mu.Unlock()

	// The app dies: its network goes with it. A new app adopts the VM, whose
	// QEMU reconnects to the rebuilt network on the same socket.
	drv.proc(d.ID).net.Close()
	drv2 := New(rtDir, devs, log)
	run2, err := drv2.Adopt(ctx, d, run.Launch, up)
	if err != nil {
		t.Fatal(err)
	}
	a = adb.Client{Bin: drv2.proc(d.ID).adb.Bin, Port: run2.Launch.ADBPort}
	c, cancel := context.WithTimeout(ctx, time.Minute)
	if err := a.WaitBoot(c, nil); err != nil {
		t.Fatalf("adb after adopt: %v", err)
	}
	cancel()
	deadline := time.Now().Add(30 * time.Second)
	for !px.saw("1.1.1.1:80") && time.Now().Before(deadline) {
		sh(`printf 'GET / HTTP/1.0\r\n\r\n' | toybox nc -w 5 1.1.1.1 80 | head -c 12`)
	}
	if !px.saw("1.1.1.1:80") {
		t.Error("no network through the proxy after adopt")
	}
	drv = drv2
	run = run2

	start = time.Now()
	if err := drv.Stop(ctx, d); err != nil {
		t.Fatal(err)
	}
	select {
	case <-run.Done:
	case <-time.After(time.Minute):
		t.Fatal("Done did not close after Stop")
	}
	t.Logf("stopped in %s", time.Since(start).Round(time.Second))

	bk := t.TempDir()
	if err := drv.Backup(ctx, d, bk); err != nil {
		t.Fatal(err)
	}
	if err := drv.Restore(ctx, d, bk); err != nil {
		t.Fatal(err)
	}
	for _, f := range imageFiles {
		if _, err := os.Stat(filepath.Join(bk, f)); err != nil {
			t.Error(err)
		}
	}
}
