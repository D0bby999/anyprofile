// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"github.com/D0bby999/anyprofile/internal/site"
	"slices"
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
	src, err := s.CreateProfile(ctx, NewProfile{Name: "Origin", Sites: []string{"etsy"}, Tags: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetAccountPassword(src.ID, "etsy", "pw-secret"); err != nil {
		t.Fatal(err)
	}
	dups, err := s.DuplicateProfile(ctx, DuplicateInput{SourceID: src.ID, Count: 1, Proxy: "none"})
	if err != nil {
		t.Fatal(err)
	}
	d := dups[0]
	if d.Name != "Origin copy" || !slices.Equal(d.Sites, []string{"etsy"}) || len(d.Tags) != 1 || d.Tags[0] != "a" {
		t.Fatalf("dup = %+v", d)
	}
	if d.SeedHex == src.SeedHex {
		t.Fatal("duplicate must get a fresh identity")
	}
	if len(d.Accounts) != 0 {
		t.Fatal("duplicate must not copy the account")
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
	if err := s.SetSitesMany(ids, []string{"nope"}); err == nil || !strings.Contains(err.Error(), "unknown_site") {
		t.Fatalf("unknown site err = %v", err)
	}
	if err := s.SetSitesMany(ids, []string{"etsy", "tiktok", "etsy"}); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		p, _ := s.Store.GetProfile(id)
		if !slices.Equal(p.Sites, []string{"etsy", "tiktok"}) {
			t.Fatalf("sites = %q", p.Sites)
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
		p, err := s.CreateProfile(ctx, NewProfile{Name: n, Sites: []string{"other"}})
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
	p.Tags = []string{"shop-us"}
	if err := s.Store.UpdateProfile(p); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAccountLogin(p.ID, "other", "alpha@example.com"); err != nil {
		t.Fatal(err)
	}
	for q, want := range map[string]string{"shop": "Alpha", "#2": "Beta", "ALPHA@": "Alpha", "gamm": "Gamma"} {
		got, _ := s.ListProfiles(store.ProfileFilter{Query: q})
		if len(got) != 1 || got[0].Name != want {
			t.Errorf("query %q = %d rows, want %s", q, len(got), want)
		}
	}
}

func TestSyncStartURLs(t *testing.T) {
	sites := []site.Site{
		{ID: "tiktok", StartURL: "https://www.tiktok.com/tiktokstudio"},
		{ID: "youtube", StartURL: "https://studio.youtube.com"},
		{ID: "shop", StartURL: "https://seller.example", StartURLs: map[string]string{"US": "https://seller-us.example", "VN": "https://seller-vn.example"}},
		{ID: "other"},
	}
	got := syncStartURLs("https://www.tiktok.com/tiktokstudio\nhttps://mine.example/", []string{"tiktok"}, []string{"youtube"}, sites, "")
	if got != "https://mine.example/\nhttps://studio.youtube.com" {
		t.Fatalf("tiktok -> youtube = %q", got)
	}
	if got := syncStartURLs("https://studio.youtube.com", []string{"youtube"}, []string{"youtube", "tiktok"}, sites, ""); got != "https://studio.youtube.com\nhttps://www.tiktok.com/tiktokstudio" {
		t.Fatalf("add tiktok = %q", got)
	}
	// A country with its own page gets it; a country without gets the default.
	if got := syncStartURLs("", nil, []string{"shop"}, sites, "us"); got != "https://seller-us.example" {
		t.Fatalf("add shop in US = %q", got)
	}
	if got := syncStartURLs("", nil, []string{"shop"}, sites, "JP"); got != "https://seller.example" {
		t.Fatalf("add shop in JP = %q", got)
	}
	// Removing the platform removes its page whichever country it was for.
	if got := syncStartURLs("https://seller-vn.example\nhttps://mine.example/", []string{"shop"}, nil, sites, "US"); got != "https://mine.example/" {
		t.Fatalf("remove shop = %q", got)
	}
}

func TestRegionStartURLs(t *testing.T) {
	sites := []site.Site{{ID: "shop", StartURL: "https://seller.example", StartURLs: map[string]string{"US": "https://seller-us.example", "VN": "https://seller-vn.example"}}}
	text := "https://seller-vn.example\nhttps://mine.example/"
	if got := regionStartURLs(text, []string{"shop"}, sites, "US"); got != "https://seller-us.example\nhttps://mine.example/" {
		t.Fatalf("VN -> US = %q", got)
	}
	if got := regionStartURLs(text, []string{"shop"}, sites, ""); got != "https://seller.example\nhttps://mine.example/" {
		t.Fatalf("VN -> unknown = %q", got)
	}
	if got := regionStartURLs(text, nil, sites, "US"); got != text {
		t.Fatalf("a page of a platform the profile is not on changed: %q", got)
	}
}

// TikTok Shop opens the seller center of the proxy's country, and follows a
// new proxy into another country; typed pages stay.
func TestStartPageFollowsProxyCountry(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if _, err := s.ImportProxies(ctx, "gw.example:10001:u:us\ngw.example:10002:u:vn", ""); err != nil {
		t.Fatal(err)
	}
	pxs, _ := s.ListProxies()
	ids := map[string]string{}
	for _, px := range pxs {
		ids[px.Country] = px.ID
	}
	p, err := s.CreateProfile(ctx, NewProfile{Name: "Shop", ProxyID: ids["US"], Sites: []string{"tiktok-shop"},
		StartURL: "https://seller.tiktok.com\nhttps://mine.example/"})
	if err != nil {
		t.Fatal(err)
	}
	if p.StartURL != "https://seller-us.tiktok.com\nhttps://mine.example/" {
		t.Fatalf("US proxy: %q", p.StartURL)
	}
	p.ProxyID = ids["VN"]
	if err := s.UpdateProfile(ctx, p); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Store.GetProfile(p.ID); got.StartURL != "https://seller-vn.tiktok.com\nhttps://mine.example/" {
		t.Fatalf("after moving to a VN proxy: %q", got.StartURL)
	}
}
