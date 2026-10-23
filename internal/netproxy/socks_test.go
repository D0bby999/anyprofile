// SPDX-License-Identifier: AGPL-3.0-or-later

package netproxy

import (
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"golang.org/x/net/proxy"
)

func echoListener(t *testing.T) net.Listener {
	t.Helper()
	ln := listen(t)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); io.Copy(c, c) }()
		}
	}()
	return ln
}

func startSOCKS(t *testing.T, up Upstream) *SOCKSRelay {
	t.Helper()
	r, err := StartSOCKS(up, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}

func TestSOCKSRelayConnects(t *testing.T) {
	up, _ := fakeHTTPProxy(t)
	echo := echoListener(t)
	r := startSOCKS(t, up)
	select {
	case <-r.Used():
		t.Fatal("used before any connection")
	default:
	}
	d, err := proxy.SOCKS5("tcp", "127.0.0.1:"+strconv.Itoa(r.Port()), nil, proxy.Direct)
	if err != nil {
		t.Fatal(err)
	}
	// By name, as hev-socks5-tunnel sends hosts with remote DNS.
	_, port, _ := net.SplitHostPort(echo.Addr().String())
	c, err := d.Dial("tcp", "localhost:"+port)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.Write([]byte("ping"))
	buf := make([]byte, 4)
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.ReadFull(c, buf); err != nil || string(buf) != "ping" {
		t.Fatalf("%q %v", buf, err)
	}
	select {
	case <-r.Used():
	case <-time.After(time.Second):
		t.Fatal("Used did not close")
	}
}

// raw sends a greeting and one request with cmd, and returns the reply code.
func raw(t *testing.T, r *SOCKSRelay, methods []byte, cmd byte) (method, rep byte) {
	t.Helper()
	c, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(r.Port()))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(5 * time.Second))
	c.Write(append([]byte{5, byte(len(methods))}, methods...))
	var g [2]byte
	if _, err := io.ReadFull(c, g[:]); err != nil {
		t.Fatal(err)
	}
	if g[1] != 0 {
		return g[1], 0
	}
	c.Write([]byte{5, cmd, 0, 1, 8, 8, 8, 8, 0, 53})
	var rp [10]byte
	if _, err := io.ReadFull(c, rp[:]); err != nil {
		t.Fatal(err)
	}
	return 0, rp[1]
}

func TestSOCKSRelayRefusesUDPBindAndAuth(t *testing.T) {
	up, _ := fakeHTTPProxy(t)
	r := startSOCKS(t, up)
	if _, rep := raw(t, r, []byte{0}, 3); rep != repCmdNotSup {
		t.Errorf("UDP ASSOCIATE: reply %d", rep)
	}
	if _, rep := raw(t, r, []byte{0}, 2); rep != repCmdNotSup {
		t.Errorf("BIND: reply %d", rep)
	}
	if m, _ := raw(t, r, []byte{2}, 1); m != 0xff {
		t.Errorf("username/password only: method %#x", m)
	}
}

func TestSOCKSRelayUpstreamDown(t *testing.T) {
	ln := listen(t)
	up := upstreamFor(ln, "http")
	ln.Close()
	r := startSOCKS(t, up)
	if _, rep := raw(t, r, []byte{0}, 1); rep != repUnreachable {
		t.Fatalf("reply %d", rep)
	}
}

func TestSOCKSRelayCloseEndsConnections(t *testing.T) {
	up, _ := fakeHTTPProxy(t)
	echo := echoListener(t)
	r, err := StartSOCKS(up, 0)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := proxy.SOCKS5("tcp", "127.0.0.1:"+strconv.Itoa(r.Port()), nil, proxy.Direct)
	c, err := d.Dial("tcp", echo.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	done := make(chan struct{})
	go func() { r.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close hangs with a connection open")
	}
	c.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := c.Read(make([]byte, 1)); err == nil {
		t.Fatal("connection still open after Close")
	}
}
