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

func TestRelayHTTPSUpstream(t *testing.T) {
	tg := target(t)
	up, denied := fakeHTTPSProxy(t)
	r, err := StartRelay(up, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if got := getVia(t, r, tg.URL+"/tls-a", true); got != "hello /tls-a" {
		t.Fatalf("HTTPS CONNECT got %q", got)
	}
	if got := getVia(t, r, tg.URL+"/tls-b", false); got != "hello /tls-b" {
		t.Fatalf("HTTPS plain got %q", got)
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
https://su:sp@tls.example.com:443
host-only
1.2.3.4:99999`)
	if len(got) != 6 {
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
	if got[3].Upstream.Scheme != "https" || got[3].Upstream.Port != 443 || got[3].Display != "https://su@tls.example.com:443" {
		t.Errorf("https form = %+v", got[3])
	}
	if got[4].Err == "" || got[5].Err == "" {
		t.Errorf("bad lines not rejected: %+v %+v", got[4], got[5])
	}
	if got[4].Code != "format" || got[5].Code != "host_port" {
		t.Errorf("codes = %q %q", got[4].Code, got[5].Code)
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
	oxy := ParseList("205.188.182.232:8001:disp_demo123:demo_pass=1\ndisp.oxylabs.io:8002:disp_demo123:pass\npr.oxylabs.io:7777:myuser:pass")
	if len(oxy) != 3 ||
		oxy[0].Upstream.Host != "disp.oxylabs.io" || oxy[0].Upstream.Username != "user-disp_demo123" || oxy[0].Upstream.Password != "demo_pass=1" ||
		oxy[1].Upstream.Host != "disp.oxylabs.io" || oxy[1].Upstream.Username != "user-disp_demo123" ||
		oxy[2].Upstream.Host != "pr.oxylabs.io" || oxy[2].Upstream.Username != "customer-myuser" {
		t.Fatalf("oxylabs normalization = %+v", oxy)
	}

	httpsList := ParseList(`
https://1.2.3.4:8443:myuser:mypass
1.2.3.4:8443:myuser:mypass:https
1.2.3.4:8443:https
https:1.2.3.4:8443:myuser:mypass
socks5://5.6.7.8:1080:s5user:s5pass
5.6.7.8:1080:s5user:s5pass:socks5
5.6.7.8:1080:socks5
https://tls.example.com:443`)
	if len(httpsList) != 8 {
		t.Fatalf("httpsList length = %d, want 8", len(httpsList))
	}
	for i, item := range httpsList[:4] {
		if item.Err != "" {
			t.Errorf("[%d] err = %v", i, item.Err)
		}
		if item.Upstream.Scheme != "https" || item.Upstream.Host != "1.2.3.4" || item.Upstream.Port != 8443 {
			t.Errorf("[%d] upstream = %+v", i, item.Upstream)
		}
		if i != 2 && (item.Upstream.Username != "myuser" || item.Upstream.Password != "mypass") {
			t.Errorf("[%d] auth = %q:%q", i, item.Upstream.Username, item.Upstream.Password)
		}
	}
	for i, item := range httpsList[4:7] {
		if item.Err != "" {
			t.Errorf("[%d] err = %v", i+4, item.Err)
		}
		if item.Upstream.Scheme != "socks5" || item.Upstream.Host != "5.6.7.8" || item.Upstream.Port != 1080 {
			t.Errorf("[%d] upstream = %+v", i+4, item.Upstream)
		}
		if i != 2 && (item.Upstream.Username != "s5user" || item.Upstream.Password != "s5pass") {
			t.Errorf("[%d] auth = %q:%q", i+4, item.Upstream.Username, item.Upstream.Password)
		}
	}
	if httpsList[7].Upstream.Scheme != "https" || httpsList[7].Upstream.Host != "tls.example.com" || httpsList[7].Upstream.Port != 443 {
		t.Errorf("[7] upstream = %+v", httpsList[7].Upstream)
	}
}
