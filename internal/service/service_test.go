// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/netproxy"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

type fakeEngine struct{ engine.Engine }

func (fakeEngine) ID() string { return "fury" }
func (fakeEngine) Releases() []engine.Release {
	return []engine.Release{{Engine: "fury", Version: "9.9.9", Catalogue: "c"}}
}
func (fakeEngine) NewIdentity(context.Context, uint64) (engine.Identity, error) {
	return engine.Identity{PersonaID: "macos-m4-1710x1112", Platform: "macos"}, nil
}
func (fakeEngine) UsesCookieKey() bool { return true }
func (fakeEngine) Status(_ context.Context, v string) engine.Status {
	return engine.Status{Version: engine.Version{Engine: "fury", Version: v}, Installed: v == "9.9.9"}
}

// fakeEngine2 stands in for a second engine that pins hardware values.
type fakeEngine2 struct{ engine.Engine }

func (fakeEngine2) ID() string { return "apostate" }
func (fakeEngine2) Releases() []engine.Release {
	return []engine.Release{{Engine: "apostate", Version: "0.5.1", Catalogue: "c"}}
}
func (fakeEngine2) Status(_ context.Context, v string) engine.Status {
	return engine.Status{Version: engine.Version{Engine: "apostate", Version: v}}
}
func (fakeEngine2) NewIdentity(context.Context, uint64) (engine.Identity, error) {
	return engine.Identity{PersonaID: "macos", Platform: "macos", Meta: store.EngineMeta{HWC: 4, MemGiB: 8}}, nil
}
func (fakeEngine2) UsesCookieKey() bool { return false }

func newService(t *testing.T) (*Service, *secrets.Memory) {
	dirs := paths.Dirs{Root: t.TempDir()}
	if err := dirs.Ensure(); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dirs.DB(), dirs.Backups())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	sec := secrets.NewMemory()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := &Service{Dirs: dirs, Store: st, Secrets: sec, Log: log,
		Engines: map[string]engine.Engine{"fury": fakeEngine{}}, Preferred: "fury",
		Launcher: launcher.New(st, sec, dirs, nil, nil, log),
		Locale: func(_ context.Context, cc string) (string, []string, error) {
			if cc == "VN" {
				return "vi", []string{"vi-VN", "vi"}, nil
			}
			return "en-US", []string{"en-US", "en"}, nil
		},
		CheckProxy: func(_ context.Context, up netproxy.Upstream) (netproxy.Exit, error) {
			lat, lon := 10.8, 106.6
			switch up.Port {
			case 823:
				return netproxy.Exit{IP: "9.9.9.9", Country: "US", Timezone: "America/Chicago", Rotating: true}, nil
			case 10002:
				return netproxy.Exit{IP: "2.2.2.2", Country: "VN", Timezone: "Asia/Ho_Chi_Minh", Lat: &lat, Lon: &lon}, nil
			}
			return netproxy.Exit{IP: "1.1.1.1", Country: "US", Timezone: "America/New_York"}, nil
		},
	}
	return s, sec
}

func TestImportAndQuickCreate(t *testing.T) {
	s, sec := newService(t)
	ctx := context.Background()
	res, err := s.ImportProxies(ctx, "gw.example:823:u:rot\nhttp://u:secret1@gw.example:10001\ngw.example:10002:u:secret2\nnot a proxy", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 3 || len(res.Failed) != 1 {
		t.Fatalf("import = %+v", res)
	}
	rows, _ := s.ListProxies()
	for _, r := range rows {
		pw, err := sec.Get(secrets.ServiceProxy, r.ID)
		if err != nil || !strings.HasPrefix(string(pw), "secret") && string(pw) != "rot" {
			t.Errorf("password for %s not in secrets: %q %v", r.Label, pw, err)
		}
		if strings.Contains(r.Label, "secret") {
			t.Errorf("label leaks password: %q", r.Label)
		}
	}

	a, err := s.QuickCreate(ctx, "Kenh A")
	if err != nil {
		t.Fatal(err)
	}
	if a.Timezone != "America/New_York" || a.UILocale != "en-US" || a.ProxyID == "" {
		t.Fatalf("first profile = %+v", a)
	}
	b, err := s.QuickCreate(ctx, "Kenh B")
	if err != nil {
		t.Fatal(err)
	}
	if b.Timezone != "Asia/Ho_Chi_Minh" || b.Languages != "vi-VN,vi" || b.Lat == nil || b.ProxyID == a.ProxyID {
		t.Fatalf("second profile = %+v", b)
	}
	c, err := s.QuickCreate(ctx, "Kenh C") // only the rotating proxy is left: go direct
	if err != nil {
		t.Fatal(err)
	}
	if c.ProxyID != "" || c.Timezone == "" {
		t.Fatalf("third profile = %+v", c)
	}
	if a.SeedHex == b.SeedHex || len(a.SeedHex) != 16 {
		t.Fatalf("seeds must be unique 16-hex: %q %q", a.SeedHex, b.SeedHex)
	}

	// Assigning the rotating proxy by hand is refused.
	var rotID string
	for _, r := range rows {
		if r.Port == 823 {
			rotID = r.ID
		}
	}
	c.ProxyID = rotID
	if err := s.UpdateProfile(ctx, c); err == nil || !strings.Contains(err.Error(), "sticky") {
		t.Fatalf("rotating proxy accepted: %v", err)
	}
}

func TestDeleteProxyAndPurge(t *testing.T) {
	s, sec := newService(t)
	ctx := context.Background()
	s.ImportProxies(ctx, "gw.example:10001:u:pw", "")
	p, err := s.QuickCreate(ctx, "Kenh")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProxy(p.ProxyID); err != nil {
		t.Fatal(err)
	}
	if _, err := sec.Get(secrets.ServiceProxy, p.ProxyID); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatal("proxy password left in secrets")
	}

	dir := s.Dirs.Profile(p.ID)
	os.MkdirAll(filepath.Join(dir, "Default"), 0o700)
	sec.Set(secrets.ServiceCookieKey, p.ID, make([]byte, 32))
	if err := s.Purge(p.ID); err == nil {
		t.Fatal("purge of a live profile must be refused")
	}
	if err := s.Trash(p.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Purge(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("profile folder survived purge")
	}
	if _, err := sec.Get(secrets.ServiceCookieKey, p.ID); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatal("cookie key survived purge")
	}
}

func TestLanguage(t *testing.T) {
	s, _ := newService(t)
	if got := s.Language(); got != "en" {
		t.Fatalf("default language = %q", got)
	}
	if err := s.SetLanguage("vi"); err != nil {
		t.Fatal(err)
	}
	if got := s.Language(); got != "vi" {
		t.Fatalf("after SetLanguage = %q", got)
	}
	err := s.SetLanguage("fr")
	if err == nil || !strings.Contains(err.Error(), "bad_language") {
		t.Fatalf("SetLanguage(fr) err = %v", err)
	}
}

func TestProfileSites(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	p, err := s.CreateProfile(ctx, NewProfile{Name: "Kênh", Sites: []string{"tiktok", "other", "youtube", "tiktok"}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p.Sites, []string{"tiktok", "youtube"}) {
		t.Fatalf("sites = %v, want each platform once and no other", p.Sites)
	}
	if q, err := s.CreateProfile(ctx, NewProfile{Name: "None"}); err != nil || !slices.Equal(q.Sites, []string{"other"}) {
		t.Fatalf("no platform = %v, %v", q.Sites, err)
	}
	many := []string{"tiktok", "youtube", "twitch", "kick", "facebook", "instagram", "x", "threads", "pinterest", "reddit", "etsy"}
	if _, err := s.CreateProfile(ctx, NewProfile{Name: "Many", Sites: many}); err == nil || !strings.Contains(err.Error(), "too_many_sites") {
		t.Fatalf("11 platforms err = %v", err)
	}
	// A cookie file from any of the profile's platforms is not a mismatch.
	doms := s.siteDomains(p.Sites)
	if siteMismatch("youtube.com", doms) != "" || siteMismatch("tiktok.com", doms) != "" || siteMismatch("etsy.com", doms) != "etsy.com" {
		t.Fatalf("domains of tiktok+youtube = %v", doms)
	}
}

func TestSites(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if _, err := s.CreateProfile(ctx, NewProfile{Name: "Etsy shop", Sites: []string{"etsy"}}); err != nil {
		t.Fatalf("create with etsy: %v", err)
	}
	if _, err := s.CreateProfile(ctx, NewProfile{Name: "Bad", Sites: []string{"nope"}}); err == nil ||
		!strings.Contains(err.Error(), "unknown_site") {
		t.Fatalf("create with nope err = %v", err)
	}
	all, err := s.Sites()
	if err != nil {
		t.Fatal(err)
	}
	builtin := 0
	for _, x := range all {
		if !x.Custom {
			builtin++
		}
	}
	if builtin != 45 {
		t.Fatalf("builtin sites = %d, want 45", builtin)
	}

	got, err := s.CreateSite(SiteInput{Name: "Tiki", Domains: []string{" https://www.Tiki.vn/abc "}, Color: "#1a94ff"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Domains) != 1 || got.Domains[0] != "tiki.vn" || got.Color != "#1A94FF" {
		t.Fatalf("created = %+v", got)
	}
	if _, err := s.CreateSite(SiteInput{Name: "X", Color: "red"}); err == nil ||
		!strings.Contains(err.Error(), "site_color") {
		t.Fatalf("bad color err = %v", err)
	}
	if _, err := s.CreateSite(SiteInput{Name: "", Color: "#123456"}); err == nil ||
		!strings.Contains(err.Error(), "site_name") {
		t.Fatalf("bad name err = %v", err)
	}
	if _, err := s.CreateSite(SiteInput{Name: "X", Domains: []string{"not a domain"}, Color: "#123456"}); err == nil ||
		!strings.Contains(err.Error(), "site_domain") {
		t.Fatalf("bad domain err = %v", err)
	}
	if _, err := s.CreateSite(SiteInput{Name: "X", StartURL: "ftp://x.y", Color: "#123456"}); err == nil ||
		!strings.Contains(err.Error(), "site_url") {
		t.Fatalf("bad url err = %v", err)
	}
	if err := s.UpdateSite("etsy", SiteInput{Name: "Hacked", Color: "#123456"}); err == nil ||
		!strings.Contains(err.Error(), "site_builtin") {
		t.Fatalf("update builtin err = %v", err)
	}
	if err := s.DeleteSite(got.ID); err != nil {
		t.Fatal(err)
	}
	if all, _ := s.Sites(); len(all) != builtin {
		t.Fatalf("after delete = %d sites", len(all))
	}
}

func TestNotifyOnWrites(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	var mu sync.Mutex
	var kinds []string
	s.OnChange = func(kind string) {
		mu.Lock()
		kinds = append(kinds, kind)
		mu.Unlock()
	}
	contains := func(k string) bool {
		mu.Lock()
		defer mu.Unlock()
		for _, x := range kinds {
			if x == k {
				return true
			}
		}
		return false
	}

	if _, err := s.ImportProxies(ctx, "gw.example:10001:u:pw", ""); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreateProfile(ctx, NewProfile{Name: "Notify"})
	if err != nil {
		t.Fatal(err)
	}
	p.Name = "Renamed"
	if err := s.UpdateProfile(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := s.Trash(p.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Restore(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateSite(SiteInput{Name: "Tiki", Color: "#1A94FF"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLanguage("vi"); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"proxies", "profiles", "sites", "settings"} {
		if !contains(k) {
			t.Fatalf("kinds %v missing %q", kinds, k)
		}
	}

	mu.Lock()
	before := len(kinds)
	mu.Unlock()
	if _, err := s.CreateProfile(ctx, NewProfile{Name: "  "}); err == nil {
		t.Fatal("empty name must fail")
	}
	mu.Lock()
	after := len(kinds)
	mu.Unlock()
	if before != after {
		t.Fatalf("failed write notified: %v", kinds)
	}
}

func TestNormalizeStartURLs(t *testing.T) {
	if _, err := normalizeStartURLs("https://a.com\nftp://x"); err == nil || !strings.Contains(err.Error(), "bad_start_url") {
		t.Fatalf("ftp err = %v", err)
	}
	got, err := normalizeStartURLs("https://a.com\n\n https://b.com \n")
	if err != nil || got != "https://a.com\nhttps://b.com" {
		t.Fatalf("normalize = %q, %v", got, err)
	}
}

func TestDefaults(t *testing.T) {
	s, _ := newService(t)
	d := s.Defaults()
	if d.Site != "other" || d.Proxy != "auto" || d.OnOpen != "start" {
		t.Fatalf("defaults = %+v", d)
	}
	if err := s.SetDefaults(Defaults{Engine: "fury", Site: "etsy", Proxy: "none", OnOpen: "restore"}); err != nil {
		t.Fatal(err)
	}
	d = s.Defaults()
	if d.Site != "etsy" || d.Proxy != "none" || d.OnOpen != "restore" {
		t.Fatalf("after set = %+v", d)
	}
	if v, _ := s.Store.GetSetting(store.SettingOnOpen); v != "restore" {
		t.Fatalf("on_open stored as %q", v)
	}
	if err := s.SetDefaults(Defaults{Engine: "fury", Site: "nope", Proxy: "auto", OnOpen: "restore"}); err == nil || !strings.Contains(err.Error(), "unknown_site") {
		t.Fatalf("unknown site err = %v", err)
	}
	if err := s.SetDefaults(Defaults{Engine: "fury", Site: "etsy", Proxy: "auto", OnOpen: "tabs"}); err == nil || !strings.Contains(err.Error(), "bad_on_open") {
		t.Fatalf("bad on-open err = %v", err)
	}
}

func TestExpiry(t *testing.T) {
	now := time.Date(2026, 10, 13, 10, 0, 0, 0, time.Local)
	if got := expiry("2026-10-15", now); got != "soon" {
		t.Fatalf("15/10 = %q", got)
	}
	if got := expiry("2026-10-17", now); got != "" {
		t.Fatalf("17/10 = %q", got)
	}
	if got := expiry("2026-10-12", now); got != "expired" {
		t.Fatalf("12/10 = %q", got)
	}
	if got := expiry("2026-10-13", now); got != "soon" {
		t.Fatalf("13/10 = %q", got)
	}
	if got := expiry("not a date", now); got != "" {
		t.Fatalf("bad date = %q", got)
	}
}

func TestProxyMeta(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if _, err := s.ImportProxies(ctx, "gw.example:10001:u:pw", "31/12/2026"); err == nil || !strings.Contains(err.Error(), "bad_date") {
		t.Fatalf("bad import date err = %v", err)
	}
	if _, err := s.ImportProxies(ctx, "gw.example:10001:u:pw", ""); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.ListProxies()
	id := rows[0].ID
	if err := s.UpdateProxy(id, ProxyMeta{Label: "us", Note: "renew soon", ExpiresOn: "2026-10-15"}); err != nil {
		t.Fatal(err)
	}
	s.Clock = func() time.Time { return time.Date(2026, 10, 13, 10, 0, 0, 0, time.Local) }
	rows, _ = s.ListProxies()
	if rows[0].Note != "renew soon" || rows[0].Expiry != "soon" {
		t.Fatalf("proxy row = %+v", rows[0])
	}
	if err := s.UpdateProxy(id, ProxyMeta{Label: "", ExpiresOn: ""}); err == nil || !strings.Contains(err.Error(), "proxy_label") {
		t.Fatalf("empty label err = %v", err)
	}
	if err := s.UpdateProxy(id, ProxyMeta{Label: "x", Note: strings.Repeat("a", 501)}); err == nil || !strings.Contains(err.Error(), "note_too_long") {
		t.Fatalf("long note err = %v", err)
	}
}

func TestDefaultEngine(t *testing.T) {
	s, _ := newService(t)
	if got := s.DefaultEngine(); got != "fury" {
		t.Fatalf("default with fury only = %q", got)
	}
	s.Engines["apostate"] = fakeEngine2{}
	s.Preferred = "apostate"
	if got := s.DefaultEngine(); got != "apostate" {
		t.Fatalf("default with preferred apostate = %q", got)
	}
	d := s.Defaults()
	d.Engine = "fury"
	if err := s.SetDefaults(d); err != nil {
		t.Fatal(err)
	}
	if got := s.DefaultEngine(); got != "fury" {
		t.Fatalf("default after saving fury = %q", got)
	}
	d.Engine = "x"
	if err := s.SetDefaults(d); err == nil || !strings.Contains(err.Error(), "bad_engine") {
		t.Fatalf("unknown engine err = %v", err)
	}
}

func TestCreateProfileEngine(t *testing.T) {
	s, _ := newService(t)
	s.Engines["apostate"] = fakeEngine2{}
	s.Preferred = "apostate"
	ctx := context.Background()
	p, err := s.CreateProfile(ctx, NewProfile{Name: "A", Sites: []string{"other"}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Engine != "apostate" || p.EngineVersion != "0.5.1" || p.EngineMeta != (store.EngineMeta{HWC: 4, MemGiB: 8}) {
		t.Fatalf("default-engine profile = %+v", p)
	}
	got, _ := s.Store.GetProfile(p.ID)
	if got.EngineMeta != (store.EngineMeta{HWC: 4, MemGiB: 8}) {
		t.Fatalf("stored meta = %+v", got.EngineMeta)
	}
	f, err := s.CreateProfile(ctx, NewProfile{Name: "F", Sites: []string{"other"}, Engine: "fury"})
	if err != nil || f.Engine != "fury" {
		t.Fatalf("fury profile = %+v %v", f, err)
	}
	if _, err := s.CreateProfile(ctx, NewProfile{Name: "X", Sites: []string{"other"}, Engine: "x"}); err == nil ||
		!strings.Contains(err.Error(), "bad_engine") {
		t.Fatalf("unknown engine err = %v", err)
	}
	if _, err := s.CreateProfiles(ctx, BulkCreate{BaseName: "B", Count: 2, Proxy: "none", Sites: []string{"other"}, Engine: "x"}); err == nil ||
		!strings.Contains(err.Error(), "bad_engine") {
		t.Fatalf("bulk unknown engine err = %v", err)
	}
}

func TestDuplicateUsesDefaultEngine(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	src, err := s.CreateProfile(ctx, NewProfile{Name: "Src", Sites: []string{"other"}})
	if err != nil || src.Engine != "fury" {
		t.Fatalf("source = %+v %v", src, err)
	}
	s.Engines["apostate"] = fakeEngine2{}
	s.Preferred = "apostate"
	out, err := s.DuplicateProfile(ctx, DuplicateInput{SourceID: src.ID, Count: 1, Proxy: "none"})
	if err != nil || len(out) != 1 || out[0].Engine != "apostate" {
		t.Fatalf("copy = %+v %v", out, err)
	}
}

func TestProxyLabelHidesUsername(t *testing.T) {
	s, _ := newService(t)
	if _, err := s.ImportProxies(context.Background(), "http://u:pw@gw.example:10001", ""); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.ListProxies()
	if len(rows) != 1 || rows[0].Label != "gw.example:10001" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestProxyCheckErrorHasCode(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if _, err := s.ImportProxies(ctx, "gw.example:10009:u:pw", ""); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.ListProxies()
	s.CheckProxy = func(context.Context, netproxy.Upstream) (netproxy.Exit, error) {
		return netproxy.Exit{}, errors.New("proxyconnect tcp: connection refused")
	}
	if _, err := s.CheckProxyByID(ctx, rows[0].ID); err == nil || !strings.HasPrefix(err.Error(), "anyprofile:proxy_unreachable:") {
		t.Fatalf("err = %v", err)
	}
	res, err := s.ImportProxies(ctx, "gw.example:10010:u:pw", "")
	if err != nil || len(res.Unreachable) != 1 || res.Unreachable[0] != "gw.example:10010" {
		t.Fatalf("import of an unreachable proxy = %+v %v", res, err)
	}
}

// A failed check is recorded and keeps the proxy from being handed out;
// the next good check clears it. The exit of the last good check stays.
func TestProxyFailureIsRecorded(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	good := s.CheckProxy
	down := func(context.Context, netproxy.Upstream) (netproxy.Exit, error) {
		return netproxy.Exit{}, errors.New("connection refused")
	}
	// The dead one is added first, as in the first-run wizard.
	s.CheckProxy = down
	if _, err := s.ImportProxies(ctx, "gw.example:20001:u:pw", ""); err != nil {
		t.Fatal(err)
	}
	s.CheckProxy = good
	if _, err := s.ImportProxies(ctx, "gw.example:20002:u:pw\ngw.example:20003:u:pw", ""); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.ListProxies()
	if rows[0].FailedAt == "" || rows[0].CheckedAt == "" || rows[0].Assignable {
		t.Fatalf("dead proxy = %+v", rows[0])
	}
	if rows[1].FailedAt != "" || !rows[1].Assignable {
		t.Fatalf("good proxy = %+v", rows[1])
	}
	if id, _ := s.nextFreeProxy(); id != rows[1].ID {
		t.Fatalf("next free = %s, want the checked one %s", id, rows[1].ID)
	}
	if n, _ := s.freeProxyCount(); n != 2 {
		t.Fatalf("free = %d, want 2", n)
	}
	if err := s.UpdateProxy(rows[1].ID, ProxyMeta{Label: rows[1].Label, ExpiresOn: "2001-01-01"}); err != nil {
		t.Fatal(err)
	}
	if id, _ := s.nextFreeProxy(); id != rows[2].ID {
		t.Fatalf("next free = %s, want the unexpired one %s", id, rows[2].ID)
	}

	// A proxy that worked and then died keeps its exit but is marked.
	s.CheckProxy = down
	if n, err := s.CheckAllProxies(ctx); err != nil || n != 3 {
		t.Fatalf("check all = %d %v, want 3 failed", n, err)
	}
	px, _ := s.Store.GetProxy(rows[2].ID)
	if px.FailedAt == "" || px.ExitIP != "1.1.1.1" || px.Timezone != "America/New_York" {
		t.Fatalf("after failure = %+v", px)
	}
	s.CheckProxy = good
	if n, err := s.CheckAllProxies(ctx); err != nil || n != 0 {
		t.Fatalf("check all = %d %v, want 0 failed", n, err)
	}
	if px, _ := s.Store.GetProxy(rows[0].ID); px.FailedAt != "" || px.ExitIP == "" {
		t.Fatalf("a good check must clear the failure: %+v", px)
	}
}

func TestOnboarding(t *testing.T) {
	s, _ := newService(t)
	if s.Onboarded() {
		t.Fatal("empty database reported onboarded")
	}
	if _, err := s.CreateProfile(context.Background(), NewProfile{Name: "Old", Sites: []string{"other"}}); err != nil {
		t.Fatal(err)
	}
	if !s.Onboarded() {
		t.Fatal("existing profile must count as onboarded")
	}
	if v, _ := s.Store.GetSetting("onboarded"); v != "1" {
		t.Fatalf("setting = %q", v)
	}

	s2, _ := newService(t)
	if err := s2.FinishOnboarding(); err != nil || !s2.Onboarded() {
		t.Fatalf("finish = %v", err)
	}
}
