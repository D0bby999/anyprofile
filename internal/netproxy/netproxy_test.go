// SPDX-License-Identifier: AGPL-3.0-or-later

package netproxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func target(t *testing.T) *httptest.Server {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hello "+r.URL.Path)
	}))
	t.Cleanup(s.Close)
	return s
}

// getVia fetches u through the relay as a browser would: CONNECT for https, absolute URI for http.
func getVia(t *testing.T, r *Relay, u string, tunnel bool) string {
	t.Helper()
	tr := &http.Transport{Proxy: http.ProxyURL(&url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", r.Port())})}
	if tunnel {
		// Force CONNECT for a plain-http target by dialing through the tunnel ourselves.
		tr = &http.Transport{DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
			return Upstream{Scheme: "http", Host: "127.0.0.1", Port: r.Port()}.DialTunnel(ctx, addr)
		}}
	}
	c := &http.Client{Transport: tr}
	defer tr.CloseIdleConnections()
	for i := 0; i < 2; i++ { // twice: the second request reuses the connection
		resp, err := c.Get(u)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if i == 1 {
			return string(b)
		}
	}
	return ""
}

func TestRelayHTTPUpstream(t *testing.T) {
	tg := target(t)
	up, denied := fakeHTTPProxy(t)
	r, err := StartRelay(up, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if got := getVia(t, r, tg.URL+"/a", true); got != "hello /a" {
		t.Fatalf("CONNECT got %q", got)
	}
	if got := getVia(t, r, tg.URL+"/b", false); got != "hello /b" {
		t.Fatalf("plain got %q", got)
	}
	if denied.Load() != 0 {
		t.Fatalf("upstream denied %d requests", denied.Load())
	}

	bad := up
	bad.Password = "wrong"
	if _, err := bad.DialTunnel(context.Background(), strings.TrimPrefix(tg.URL, "http://")); err != ErrUpstreamAuth {
		t.Fatalf("wrong password err = %v", err)
	}
}

func TestRelaySOCKS5Upstream(t *testing.T) {
	tg := target(t)
	r, err := StartRelay(fakeSOCKS5(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if got := getVia(t, r, tg.URL+"/s", true); got != "hello /s" {
		t.Fatalf("CONNECT got %q", got)
	}
	if got := getVia(t, r, tg.URL+"/p", false); got != "hello /p" {
		t.Fatalf("plain got %q", got)
	}
}

func TestRelayCloseDropsConnections(t *testing.T) {
	up, _ := fakeHTTPProxy(t)
	r, err := StartRelay(up, 0)
	if err != nil {
		t.Fatal(err)
	}
	port := r.Port()
	r.Close()
	if r2, err := StartRelay(up, port); err != nil {
		t.Fatalf("port not released: %v", err)
	} else {
		r2.Close()
	}
}

func TestCheckThroughProxy(t *testing.T) {
	var n atomic.Int32
	geo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json":
			fmt.Fprint(w, `{"ip":"1.2.3.4","country":"VN","city":"Hanoi","loc":"21.0,105.8","timezone":"Asia/Bangkok"}`)
		case "/ip-sticky":
			fmt.Fprint(w, "1.2.3.4")
		case "/ip-rotating":
			fmt.Fprintf(w, "5.6.7.%d", n.Add(1))
		case "/ip64-v6":
			fmt.Fprint(w, "2001:db8::7")
		case "/at/2001:db8::7/json":
			fmt.Fprint(w, `{"ip":"2001:db8::7","country":"US","timezone":"America/Chicago"}`)
		}
	}))
	defer geo.Close()
	old := [5]string{ipinfoURL, ipwhoURL, ipifyURL, ip64URL, ipinfoAt}
	defer func() { ipinfoURL, ipwhoURL, ipifyURL, ip64URL, ipinfoAt = old[0], old[1], old[2], old[3], old[4] }()
	ipinfoURL, ipwhoURL, ipinfoAt = geo.URL+"/json", geo.URL+"/none", geo.URL+"/at/%s/json"
	ip64URL = geo.URL + "/ip-sticky" // no IPv6: the same address

	up, _ := fakeHTTPProxy(t)
	ipifyURL = geo.URL + "/ip-sticky"
	e, err := Check(context.Background(), up)
	if err != nil {
		t.Fatal(err)
	}
	if e.IP != "1.2.3.4" || e.Timezone != "Asia/Ho_Chi_Minh" || e.Lat == nil || *e.Lat != 21.0 || e.Rotating || e.IPv6 != "" {
		t.Fatalf("exit = %+v", e)
	}
	// Sites with IPv6 see another address, in another place.
	ip64URL = geo.URL + "/ip64-v6"
	if e, _ := Check(context.Background(), up); e.IPv6 != "2001:db8::7" || e.IPv6Timezone != "America/Chicago" {
		t.Fatalf("ipv6 exit = %q %q", e.IPv6, e.IPv6Timezone)
	}
	ip64URL = geo.URL + "/ip-sticky"
	ipifyURL = geo.URL + "/ip-rotating"
	if e, _ := Check(context.Background(), up); !e.Rotating {
		t.Fatal("rotating proxy not flagged")
	}
}

func TestNormalizeTimezone(t *testing.T) {
	cases := [][3]string{
		{"VN", "Asia/Bangkok", "Asia/Ho_Chi_Minh"},
		{"vn", "Asia/Saigon", "Asia/Ho_Chi_Minh"},
		{"US", "America/New_York", "America/New_York"},
		{"", "Asia/Calcutta", "Asia/Kolkata"},
		{"US", "Mars/Olympus", ""},
		{"BR", "", ""},
	}
	for _, c := range cases {
		if got := NormalizeTimezone(c[0], c[1]); got != c[2] {
			t.Errorf("NormalizeTimezone(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

func TestParseList(t *testing.T) {
	got := ParseList(`
# comment
http://u:p@gw.example.com:10001
socks5h://a:b@1.2.3.4:1080
gw.example.com:823:user:pa:ss
host-only
1.2.3.4:99999`)
	if len(got) != 5 {
		t.Fatalf("lines = %d", len(got))
	}
	if got[0].Upstream.Port != 10001 || got[0].Upstream.Password != "p" || got[0].Display != "http://u@gw.example.com:10001" {
		t.Errorf("url form = %+v", got[0])
	}
	if got[1].Upstream.Scheme != "socks5" {
		t.Errorf("socks5h = %+v", got[1])
	}
	if got[2].Upstream.Password != "pa:ss" || got[2].Upstream.Scheme != "http" {
		t.Errorf("colon form = %+v", got[2])
	}
	if got[3].Err == "" || got[4].Err == "" {
		t.Errorf("bad lines not rejected: %+v %+v", got[3], got[4])
	}
	if got[3].Code != "format" || got[4].Code != "host_port" {
		t.Errorf("codes = %q %q", got[3].Code, got[4].Code)
	}
	for line, code := range map[string]string{"ftp://h:21": "scheme", "http://:80": "host_port", "h:port": "port", "proxy cua toi": "format"} {
		if g := ParseList(line); len(g) != 1 || g[0].Code != code {
			t.Errorf("%q: %+v, want code %s", line, g, code)
		}
	}
	for _, p := range got {
		if strings.Contains(p.Display, "pa") && strings.Contains(p.Display, "ss") {
			t.Errorf("display leaks password: %q", p.Display)
		}
	}
}
