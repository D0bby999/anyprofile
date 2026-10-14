// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"

	"github.com/D0bby999/anyprofile/internal/netproxy"

	"github.com/D0bby999/anyprofile/internal/store"
)

func TestExitFlags(t *testing.T) {
	prof := store.Profile{ProxyID: "px", Timezone: "America/New_York"}
	us := &store.Session{ID: 1, ProxyID: "px", ExitIP: "1.1.1.1", ExitCountry: "US", ExitTZ: "America/New_York"}
	ids := func(fs []Flag) (out []string) {
		for _, f := range fs {
			out = append(out, f.ID+":"+f.Level)
		}
		return out
	}
	cases := []struct {
		name             string
		prof             store.Profile
		latest, previous *store.Session
		want             []string
	}{
		{"same exit", prof, us, us, nil},
		{"country changed", prof, &store.Session{ID: 2, ProxyID: "px", ExitIP: "2.2.2.2", ExitCountry: "DE", ExitTZ: "Europe/Berlin"}, us,
			[]string{"country_changed:danger", "tz_mismatch:warn"}},
		{"ip changed", prof, &store.Session{ID: 2, ProxyID: "px", ExitIP: "1.1.1.2", ExitCountry: "US", ExitTZ: "America/New_York"}, us,
			[]string{"ip_changed:info"}},
		{"deprecated zone name is the same zone", store.Profile{ProxyID: "px", Timezone: "Asia/Ho_Chi_Minh"},
			&store.Session{ID: 1, ProxyID: "px", ExitIP: "3.3.3.3", ExitCountry: "VN", ExitTZ: "Asia/Saigon"}, nil, nil},
		{"tz mismatch", store.Profile{ProxyID: "px", Timezone: "Asia/Taipei"}, us, nil, []string{"tz_mismatch:warn"}},
		{"exit failed", prof, &store.Session{ID: 2, ProxyID: "px", ExitError: "timeout"}, us, []string{"exit_failed:warn"}},
		{"only one session", prof, us, &store.Session{}, nil},
		{"another proxy since last time", prof, &store.Session{ID: 2, ProxyID: "py", ExitIP: "2.2.2.2", ExitCountry: "DE", ExitTZ: "America/New_York"}, us, nil},
		{"no proxy", store.Profile{Timezone: "UTC"}, us, us, []string{"direct:info"}},
		{"never measured", prof, &store.Session{}, &store.Session{}, nil},
	}
	for _, c := range cases {
		got := exitFlags(c.prof, c.latest, c.previous)
		if got == nil {
			t.Fatalf("%s: flags must never be nil", c.name)
		}
		if g := ids(got); len(g) != len(c.want) || (len(g) > 0 && !equal(g, c.want)) {
			t.Errorf("%s: flags = %v, want %v", c.name, g, c.want)
		}
	}
	if f := exitFlags(prof, &store.Session{ID: 2, ProxyID: "px", ExitIP: "2.2.2.2", ExitCountry: "DE"}, us)[0]; f.Params["from"] != "US" || f.Params["to"] != "DE" {
		t.Fatalf("params = %v", f.Params)
	}
}

func equal(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMatchExit(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if _, err := s.ImportProxies(ctx, "gw.example:10001:u:pw", ""); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreateProfile(ctx, NewProfile{Name: "A", Sites: []string{"other"}, ProxyID: AutoProxy})
	if err != nil || p.Timezone != "America/New_York" {
		t.Fatalf("created = %+v %v", p, err)
	}
	// The sticky port now exits in Vietnam.
	s.CheckProxy = func(context.Context, netproxy.Upstream) (netproxy.Exit, error) {
		return netproxy.Exit{IP: "2.2.2.2", Country: "VN", Timezone: "Asia/Ho_Chi_Minh"}, nil
	}
	if err := s.MatchExit(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Store.GetProfile(p.ID)
	if got.Timezone != "Asia/Ho_Chi_Minh" || got.Languages != "vi-VN,vi" {
		t.Fatalf("after match = %s %s", got.Timezone, got.Languages)
	}
}
