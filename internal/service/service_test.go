// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/netproxy"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

type fakeEngine struct{ engine.Engine }

func (fakeEngine) ID() string             { return "fury" }
func (fakeEngine) Pinned() engine.Version { return engine.Version{Engine: "fury", Version: "9.9.9"} }
func (fakeEngine) NewIdentity(context.Context, uint64) (string, string, error) {
	return "macos-m4-1710x1112", "macos", nil
}

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
	s := &Service{Dirs: dirs, Store: st, Secrets: sec, Engine: fakeEngine{}, Log: log,
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
	res, err := s.ImportProxies(ctx, "gw.example:823:u:rot\nhttp://u:secret1@gw.example:10001\ngw.example:10002:u:secret2\nnot a proxy")
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
	s.ImportProxies(ctx, "gw.example:10001:u:pw")
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

func TestSites(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if _, err := s.CreateProfile(ctx, NewProfile{Name: "Etsy shop", Site: "etsy"}); err != nil {
		t.Fatalf("create with etsy: %v", err)
	}
	if _, err := s.CreateProfile(ctx, NewProfile{Name: "Bad", Site: "nope"}); err == nil ||
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
