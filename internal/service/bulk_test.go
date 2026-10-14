// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/D0bby999/anyprofile/internal/store"
)

func TestNameSeries(t *testing.T) {
	if got := nameSeries("Shop", 1); len(got) != 1 || got[0] != "Shop" {
		t.Fatalf("n=1 = %v", got)
	}
	got := nameSeries("Shop", 3)
	if len(got) != 3 || got[0] != "Shop 1" || got[1] != "Shop 2" || got[2] != "Shop 3" {
		t.Fatalf("n=3 = %v", got)
	}
}

func TestBulkCreateAndDuplicate(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if _, err := s.ImportProxies(ctx, "gw.example:10001:u:pw1\ngw.example:10003:u:pw2", ""); err != nil {
		t.Fatal(err)
	}

	// Not enough proxies: fail before creating anything.
	if _, err := s.CreateProfiles(ctx, BulkCreate{BaseName: "X", Count: 3, Proxy: "auto"}); err == nil ||
		!strings.Contains(err.Error(), "not_enough_proxies") {
		t.Fatalf("3 auto on 2 proxies err = %v", err)
	}
	if rows, _ := s.ListProfiles(store.ProfileFilter{}); len(rows) != 0 {
		t.Fatalf("no profiles must exist after a failed bulk: %d", len(rows))
	}

	// Two profiles, one free proxy each.
	created, err := s.CreateProfiles(ctx, BulkCreate{BaseName: "Shop", Count: 2, Proxy: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 || created[0].ProxyID == "" || created[1].ProxyID == "" || created[0].ProxyID == created[1].ProxyID {
		t.Fatalf("created = %+v", created)
	}

	// Duplicate copies settings, never identity or account.
	src, err := s.CreateProfile(ctx, NewProfile{Name: "Origin", Site: "etsy", Tags: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetAccountPassword(src.ID, "pw-secret"); err != nil {
		t.Fatal(err)
	}
	dups, err := s.DuplicateProfile(ctx, DuplicateInput{SourceID: src.ID, Count: 1, Proxy: "none"})
	if err != nil {
		t.Fatal(err)
	}
	d := dups[0]
	if d.Name != "Origin copy" || d.Site != "etsy" || len(d.Tags) != 1 || d.Tags[0] != "a" {
		t.Fatalf("dup = %+v", d)
	}
	if d.SeedHex == src.SeedHex {
		t.Fatal("duplicate must get a fresh identity")
	}
	if d.HasPassword {
		t.Fatal("duplicate must not copy the password")
	}
	if d.ProxyID != "" {
		t.Fatalf("dup proxy = %q, want none", d.ProxyID)
	}
}

func TestBulkEdits(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if _, err := s.ImportProxies(ctx, "gw.example:10001:u:pw1\ngw.example:10003:u:pw2", ""); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, n := range []string{"a", "b", "c"} {
		p, err := s.CreateProfile(ctx, NewProfile{Name: n})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, p.ID)
	}

	// Add the same tag twice without duplicating.
	if err := s.AddTagMany(ids[:2], "batch"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddTagMany(ids[:2], "batch"); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids[:2] {
		p, _ := s.Store.GetProfile(id)
		if len(p.Tags) != 1 || p.Tags[0] != "batch" {
			t.Fatalf("tags = %v", p.Tags)
		}
	}

	// Unknown platform is refused.
	if err := s.SetSiteMany(ids, "nope"); err == nil || !strings.Contains(err.Error(), "unknown_site") {
		t.Fatalf("unknown site err = %v", err)
	}
	if err := s.SetSiteMany(ids, "etsy"); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		p, _ := s.Store.GetProfile(id)
		if p.Site != "etsy" {
			t.Fatalf("site = %q", p.Site)
		}
	}

	// Three profiles, two free proxies: the third reports not_enough_proxies.
	errs := s.AssignProxiesMany(ctx, ids, "auto")
	if len(errs) != 1 {
		t.Fatalf("assign errs = %v", errs)
	}
	for id, msg := range errs {
		if !strings.Contains(msg, "not_enough_proxies") {
			t.Fatalf("unexpected error for %s: %s", id, msg)
		}
	}
	got := map[string]bool{}
	for _, id := range ids {
		p, _ := s.Store.GetProfile(id)
		if p.ProxyID != "" {
			got[p.ProxyID] = true
		}
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 distinct proxies, got %v", got)
	}
}

func TestPinAndSearch(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	var ids []string
	for _, n := range []string{"Alpha", "Beta", "Gamma"} {
		p, err := s.CreateProfile(ctx, NewProfile{Name: n, Site: "other"})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, p.ID)
	}
	if err := s.SetPinnedMany([]string{ids[1], ids[2]}, true); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.ListProfiles(store.ProfileFilter{})
	if len(rows) != 3 || !rows[0].Pinned || !rows[1].Pinned || rows[2].Pinned || rows[2].ID != ids[0] {
		t.Fatalf("pinned order = %v", []string{rows[0].Name, rows[1].Name, rows[2].Name})
	}

	p, _ := s.Store.GetProfile(ids[0])
	p.Tags, p.Login = []string{"shop-us"}, "alpha@example.com"
	if err := s.Store.UpdateProfile(p); err != nil {
		t.Fatal(err)
	}
	for q, want := range map[string]string{"shop": "Alpha", "#2": "Beta", "ALPHA@": "Alpha", "gamm": "Gamma"} {
		got, _ := s.ListProfiles(store.ProfileFilter{Query: q})
		if len(got) != 1 || got[0].Name != want {
			t.Errorf("query %q = %d rows, want %s", q, len(got), want)
		}
	}
}
