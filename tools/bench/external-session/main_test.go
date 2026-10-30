// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/engine/anybrowser"
	"github.com/D0bby999/anyprofile/internal/netproxy"
	"github.com/D0bby999/anyprofile/internal/store"
)

func descriptorFixture() engine.Release {
	rel := anybrowser.Builtin()[0]
	rel.URL = "https://example.com/candidate.zip"
	rel.SHA256 = strings.Repeat("a", 64)
	for file := range rel.Files {
		rel.Files[file] = strings.Repeat("b", 64)
	}
	return rel
}

func TestSameProxyExitRejectsLongWindowDrift(t *testing.T) {
	expected := store.Proxy{ExitIP: "198.51.100.1", Country: "US", Timezone: "America/Chicago"}
	stable := netproxy.Exit{IP: expected.ExitIP, Country: expected.Country, Timezone: expected.Timezone}
	if !sameProxyExit(expected, stable) {
		t.Fatal("stable observed exit rejected")
	}
	for name, change := range map[string]func(*netproxy.Exit){
		"new IP":       func(e *netproxy.Exit) { e.IP = "198.51.100.2" },
		"unknown IP":   func(e *netproxy.Exit) { e.IP = "" },
		"new IPv6":     func(e *netproxy.Exit) { e.IPv6 = "2001:db8::1" },
		"new country":  func(e *netproxy.Exit) { e.Country = "MA" },
		"new timezone": func(e *netproxy.Exit) { e.Timezone = "Africa/Casablanca" },
		"rotating":     func(e *netproxy.Exit) { e.Rotating = true },
	} {
		t.Run(name, func(t *testing.T) {
			exit := stable
			change(&exit)
			if sameProxyExit(expected, exit) {
				t.Fatal("changed/unknown proxy exit accepted")
			}
		})
	}
	if sameProxyExit(store.Proxy{}, netproxy.Exit{}) {
		t.Fatal("empty observations are not proof of stability")
	}
	for name, mutate := range map[string]func(*store.Proxy, *netproxy.Exit){
		"missing country":  func(p *store.Proxy, e *netproxy.Exit) { p.Country, e.Country = "", "" },
		"missing timezone": func(p *store.Proxy, e *netproxy.Exit) { p.Timezone, e.Timezone = "", "" },
	} {
		t.Run(name, func(t *testing.T) {
			p, e := expected, stable
			mutate(&p, &e)
			if sameProxyExit(p, e) {
				t.Fatal("incomplete identity accepted")
			}
		})
	}
}

func TestMonitorExitInvalidatesOnFailure(t *testing.T) {
	for _, failure := range []string{"exit drift", "health unavailable"} {
		t.Run(failure, func(t *testing.T) {
			ticks := make(chan time.Time, 2)
			ticks <- time.Now()
			ticks <- time.Now()
			calls := 0
			err := monitorExit(context.Background(), ticks, func(ctx context.Context) error {
				calls++
				if _, ok := ctx.Deadline(); !ok {
					t.Error("health check has no deadline")
				}
				if calls == 2 {
					return errors.New(failure)
				}
				return nil
			})
			if err == nil || err.Error() != failure || calls != 2 {
				t.Fatalf("monitor did not stop on failure: calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestMonitorExitStopsOnCancellationOrLostTicker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := monitorExit(ctx, make(chan time.Time), func(context.Context) error {
		t.Error("check ran after cancellation")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ticks := make(chan time.Time)
	close(ticks)
	if err := monitorExit(context.Background(), ticks, func(context.Context) error { return nil }); err == nil {
		t.Fatal("lost monitor ticker accepted")
	}
}

func TestReadProxyFilePrivateAndRedacted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX private-file fixture; Windows proxy tests require native ACL handling")
	}
	const secret = "test-password-not-for-logs"
	path := filepath.Join(t.TempDir(), "proxy")
	value := "http://fixture:" + secret + "@127.0.0.1:10000"
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := readProxyFile(path); err != nil || got != value {
		t.Fatal("private valid proxy was not accepted")
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readProxyFile(path); err == nil || strings.Contains(err.Error(), secret) {
		t.Fatal("non-private proxy file accepted or credential leaked")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{value + "\n" + value, "invalid:" + secret} {
		if err := os.WriteFile(path, []byte(invalid), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readProxyFile(path); err == nil || strings.Contains(err.Error(), secret) {
			t.Fatal("invalid proxy input accepted or credential leaked")
		}
	}
}

func TestExactCandidateDescriptor(t *testing.T) {
	if err := validateDescriptor(descriptorFixture()); err != nil {
		t.Fatal(err)
	}
}

func TestDescriptorCannotBypassIdentityOrCoreCoverage(t *testing.T) {
	for name, mutate := range map[string]func(*engine.Release){
		"wrong version": func(r *engine.Release) { r.Version = "999.1" },
		"wrong origin":  func(r *engine.Release) { r.Engine = "fury" },
		"wrong cohort":  func(r *engine.Release) { r.Catalogue = strings.Repeat("c", 64) },
		"missing core":  func(r *engine.Release) { delete(r.Files, r.Bin) },
		"zero core":     func(r *engine.Release) { r.Files[r.Bin] = strings.Repeat("0", 64) },
	} {
		t.Run(name, func(t *testing.T) {
			rel := descriptorFixture()
			mutate(&rel)
			if err := validateDescriptor(rel); err == nil {
				t.Fatal("invalid descriptor accepted")
			}
		})
	}
}
