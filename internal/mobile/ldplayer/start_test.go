// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package ldplayer

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/proxy"

	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/netproxy"
)

// fakeADB is a shell script standing in for adb: it logs its arguments and
// answers what the driver asks.
const fakeADB = `#!/bin/sh
echo "$@" >> "$AP_ADB_LOG"
case "$*" in
  *"getprop sys.boot_completed"*) echo 1 ;;
  *"id -u"*) echo 0 ;;
  *"pm path hev.sockstun"*) echo package:/data/app/hev.sockstun/base.apk ;;
  *"always_on_vpn_lockdown 1"*) touch "$AP_ADB_VPN" ;;
esac
exit 0
`

// connectProxy is an upstream HTTP proxy that records its targets.
type connectProxy struct {
	mu      sync.Mutex
	targets []string
}

func (p *connectProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	p.targets = append(p.targets, r.Host)
	p.mu.Unlock()
	out, err := net.Dial("tcp", r.Host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	in, buf, _ := w.(http.Hijacker).Hijack()
	in.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
	go func() { io.Copy(out, buf); out.Close() }()
	io.Copy(in, out)
	in.Close()
}

func setupADB(t *testing.T, d *Driver) (log, vpn string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "adb")
	if err := os.WriteFile(bin, []byte(fakeADB), 0o700); err != nil {
		t.Fatal(err)
	}
	log, vpn = filepath.Join(dir, "adb.log"), filepath.Join(dir, "vpn")
	t.Setenv("AP_ADB_LOG", log)
	t.Setenv("AP_ADB_VPN", vpn)
	d.find = func() (Install, error) { return Install{Dir: dir, Console: "ldconsole", ADB: bin}, nil }
	d.poll = 20 * time.Millisecond
	return log, vpn
}

var reverseRe = regexp.MustCompile(`reverse tcp:(\d+) tcp:\d+`)

// android plays SocksTun: once the VPN is on, it connects through the relay.
func android(t *testing.T, log, vpn, target string) {
	for i := 0; i < 500; i++ {
		if _, err := os.Stat(vpn); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	b, _ := os.ReadFile(log)
	m := reverseRe.FindSubmatch(b)
	if m == nil {
		t.Error("no adb reverse before the VPN")
		return
	}
	d, _ := proxy.SOCKS5("tcp", "127.0.0.1:"+string(m[1]), nil, proxy.Direct)
	c, err := d.Dial("tcp", target)
	if err != nil {
		t.Error(err)
		return
	}
	c.Close()
}

func TestStartThroughTunnel(t *testing.T) {
	d, fc := newFake(t)
	log, vpn := setupADB(t, d)
	fc.list = []Instance{{Index: 2, Name: "ap-abc", PID: 77}}
	px := &connectProxy{}
	pl, _ := net.Listen("tcp", "127.0.0.1:0")
	go http.Serve(pl, px)
	defer pl.Close()
	echo, _ := net.Listen("tcp", "127.0.0.1:0")
	defer echo.Close()
	go func() {
		for {
			c, err := echo.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	up := &netproxy.Upstream{Scheme: "http", Host: "127.0.0.1", Port: pl.Addr().(*net.TCPAddr).Port}
	go android(t, log, vpn, echo.Addr().String())

	run, err := d.Start(context.Background(), mobile.Device{ID: "abc", Ref: "ap-abc", RAMMB: 3072, CPUs: 4},
		mobile.StartOptions{Upstream: up, Timezone: "Asia/Tokyo", Locale: "ja-JP"})
	if err != nil {
		t.Fatal(err)
	}
	if run.Launch.ADBPort != 5559 || run.Launch.RelayPort == 0 || run.Launch.PID != 77 {
		t.Fatalf("launch %+v", run.Launch)
	}
	px.mu.Lock()
	saw := strings.Join(px.targets, " ")
	px.mu.Unlock()
	if !strings.Contains(saw, echo.Addr().String()) {
		t.Fatalf("upstream saw %q", saw)
	}
	b, _ := os.ReadFile(log)
	got := string(b)
	for _, w := range []string{"-s 127.0.0.1:5559", "reverse tcp:" + strconv.Itoa(run.Launch.RelayPort), "push ",
		"setprop persist.sys.timezone Asia/Tokyo", "setprop persist.sys.locale ja-JP"} {
		if !strings.Contains(got, w) {
			t.Errorf("adb never got %q", w)
		}
	}
	if strings.Contains(got, " install ") {
		t.Error("installed the tunnel app although it was there")
	}

	// LDPlayer closes the instance: Done closes, the relay goes.
	fc.mu.Lock()
	fc.running["ap-abc"] = false
	fc.mu.Unlock()
	select {
	case <-run.Done:
	case <-time.After(5 * time.Second):
		t.Fatal("Done did not close")
	}
	if c, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(run.Launch.RelayPort)); err == nil {
		c.Close()
		t.Fatal("relay still listening")
	}
}

func TestStartTunnelNeverConnects(t *testing.T) {
	d, fc := newFake(t)
	setupADB(t, d)
	old := tunnelTimeout
	tunnelTimeout = 200 * time.Millisecond
	defer func() { tunnelTimeout = old }()
	fc.list = []Instance{{Index: 0, Name: "ap-abc"}}
	up := &netproxy.Upstream{Scheme: "http", Host: "127.0.0.1", Port: 9}
	_, err := d.Start(context.Background(), mobile.Device{ID: "abc", Ref: "ap-abc"}, mobile.StartOptions{Upstream: up})
	if err == nil || !strings.Contains(err.Error(), "tunnel") {
		t.Fatalf("%v", err)
	}
	if fc.running["ap-abc"] {
		t.Fatal("left running without a working tunnel")
	}
}

func TestStartWithoutProxyTurnsVPNOff(t *testing.T) {
	d, fc := newFake(t)
	log, _ := setupADB(t, d)
	fc.list = []Instance{{Index: 0, Name: "ap-abc"}}
	if _, err := d.Start(context.Background(), mobile.Device{ID: "abc", Ref: "ap-abc"}, mobile.StartOptions{}); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Open(log)
	defer f.Close()
	off := false
	for s := bufio.NewScanner(f); s.Scan(); {
		off = off || strings.Contains(s.Text(), "always_on_vpn_lockdown 0")
	}
	if !off {
		t.Fatal("a device without a proxy kept the VPN lockdown")
	}
}
