// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build network

package netproxy

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Real-proxy checks. Secrets come from the environment of the test process only:
//
//	ANYPROFILE_TEST_PROXY=$(security find-generic-password -s anyprofile-test-proxy -w) \
//	ANYPROFILE_TEST_PROXY_ROTATING=$(security find-generic-password -s anyprofile-test-proxy-rotating -w) \
//	go test -tags network -run Real -v ./internal/netproxy/
func realUpstream(t *testing.T, env string) Upstream {
	t.Helper()
	raw := os.Getenv(env)
	if raw == "" {
		t.Skipf("%s not set", env)
	}
	up, err := parseOne(raw)
	if err != nil {
		t.Fatalf("%s: %v", env, err)
	}
	return up
}

func TestRealStickyProxy(t *testing.T) {
	up := realUpstream(t, "ANYPROFILE_TEST_PROXY")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	e, err := Check(ctx, up)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s -> %s %s %s %dms rotating=%v via %s", up.Redacted(), e.IP, e.Country, e.Timezone, e.LatencyMs, e.Rotating, e.Source)
	if e.Rotating || e.Timezone == "" || e.Lat == nil {
		t.Fatalf("sticky port should keep one IP and resolve geo: %+v", e)
	}

	// The browser only ever sees the credential-free loopback relay.
	r, err := StartRelay(up, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	relay, _ := url.Parse("http://127.0.0.1:" + strconv.Itoa(r.Port()))
	c := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(relay)}}
	for _, target := range []string{"https://api.ipify.org", "http://api.ipify.org"} {
		resp, err := c.Get(target)
		if err != nil {
			t.Fatalf("%s via relay: %v", target, err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		got := strings.TrimSpace(string(b))
		t.Logf("relay %s -> %s", target, got)
		if got != e.IP {
			t.Fatalf("relay exit %q != checked exit %q", got, e.IP)
		}
	}
}

func TestRealRotatingProxy(t *testing.T) {
	up := realUpstream(t, "ANYPROFILE_TEST_PROXY_ROTATING")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	e, err := Check(ctx, up)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s -> %s %s %s %dms rotating=%v", up.Redacted(), e.IP, e.Country, e.Timezone, e.LatencyMs, e.Rotating)
	if !e.Rotating {
		t.Fatalf("rotating gateway was not detected as rotating")
	}
}
