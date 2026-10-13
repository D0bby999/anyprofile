// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

// ProfileRow is a profile plus what the table shows beside it.
type ProfileRow struct {
	store.Profile
	State        launcher.State `json:"state"`
	StateError   string         `json:"stateError"`
	CDPPort      int            `json:"cdpPort"`
	ProxyLabel   string         `json:"proxyLabel"`
	ProxyCountry string         `json:"proxyCountry"`
	ProxyExitIP  string         `json:"proxyExitIp"`
}

// AutoProxy as NewProfile.ProxyID picks the next free sticky proxy (none if
// every proxy is taken).
const AutoProxy = "auto"

// NewProfile is what the create form sends. Identity is never chosen by hand.
type NewProfile struct {
	Name     string   `json:"name"`
	ProxyID  string   `json:"proxyId"`
	Site     string   `json:"site"`
	StartURL string   `json:"startUrl"`
	Notes    string   `json:"notes"`
	Tags     []string `json:"tags"`
}

func (s *Service) ListProfiles(f store.ProfileFilter) ([]ProfileRow, error) {
	ps, err := s.Store.ListProfiles(f)
	if err != nil {
		return nil, err
	}
	proxies, err := s.Store.ListProxies()
	if err != nil {
		return nil, err
	}
	byID := map[string]store.Proxy{}
	for _, p := range proxies {
		byID[p.ID] = p
	}
	running := map[string]launcher.Info{}
	for _, in := range s.Launcher.List() {
		running[in.ProfileID] = in
	}
	out := make([]ProfileRow, 0, len(ps))
	for _, p := range ps {
		row := ProfileRow{Profile: p, State: launcher.Stopped}
		if in, ok := running[p.ID]; ok {
			row.State, row.StateError, row.CDPPort = in.State, in.Error, in.CDPPort
		}
		if px, ok := byID[p.ProxyID]; ok {
			row.ProxyLabel, row.ProxyCountry, row.ProxyExitIP = px.Label, px.Country, px.ExitIP
		}
		out = append(out, row)
	}
	return out, nil
}

// QuickCreate makes a profile from a name alone, on the next free proxy.
func (s *Service) QuickCreate(ctx context.Context, name string) (store.Profile, error) {
	return s.CreateProfile(ctx, NewProfile{Name: name, ProxyID: AutoProxy})
}

// normalizeStartURLs checks each line is an http(s) URL; at most 10.
func normalizeStartURLs(s string) (string, error) {
	var keep []string
	for _, l := range (store.Profile{StartURL: s}).StartURLs() {
		u, err := url.Parse(l)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return "", apperr.Newf("bad_start_url", "%q is not an http(s) URL", l)
		}
		keep = append(keep, l)
	}
	if len(keep) > 10 {
		return "", apperr.New("too_many_start_urls", "at most 10 start pages")
	}
	return strings.Join(keep, "\n"), nil
}

func (s *Service) CreateProfile(ctx context.Context, in NewProfile) (store.Profile, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return store.Profile{}, apperr.New("name_required", "profile name is required")
	}
	if err := s.checkSite(in.Site); err != nil {
		return store.Profile{}, err
	}
	if in.ProxyID == AutoProxy {
		id, err := s.nextFreeProxy()
		if err != nil {
			return store.Profile{}, err
		}
		in.ProxyID = id
	}
	persona, platform, err := s.Engine.NewIdentity(ctx, randomU64())
	if err != nil {
		return store.Profile{}, err
	}
	startURL, err := normalizeStartURLs(in.StartURL)
	if err != nil {
		return store.Profile{}, err
	}
	p := store.Profile{
		ID: randomHex(8), Name: in.Name, Engine: s.Engine.ID(), EngineVersion: s.Engine.Pinned().Version,
		PersonaID: persona, SeedHex: randomHex(8), Platform: platform, ProxyID: in.ProxyID,
		Site: in.Site, StartURL: startURL, Notes: in.Notes, Tags: in.Tags,
	}
	if err := s.applyNetwork(ctx, &p); err != nil {
		return store.Profile{}, err
	}
	if err := s.Store.CreateProfile(&p); err != nil {
		return store.Profile{}, err
	}
	s.notify("profiles")
	return p, nil
}

// UpdateProfile saves editable fields; a new proxy re-derives timezone,
// position and languages so the profile keeps matching its exit.
func (s *Service) UpdateProfile(ctx context.Context, p store.Profile) error {
	if err := s.checkSite(p.Site); err != nil {
		return err
	}
	cur, err := s.Store.GetProfile(p.ID)
	if err != nil {
		return err
	}
	if p.ProxyID != cur.ProxyID {
		if err := s.applyNetwork(ctx, &p); err != nil {
			return err
		}
	}
	startURL, err := normalizeStartURLs(p.StartURL)
	if err != nil {
		return err
	}
	p.StartURL = startURL
	if err := s.Store.UpdateProfile(p); err != nil {
		return err
	}
	s.notify("profiles")
	return nil
}

// applyNetwork fills timezone, position and languages from the profile's proxy exit.
func (s *Service) applyNetwork(ctx context.Context, p *store.Profile) error {
	country := ""
	p.Lat, p.Lon = nil, nil
	if p.ProxyID == "" {
		p.Timezone = localZone()
	} else {
		px, err := s.Store.GetProxy(p.ProxyID)
		if err != nil {
			return fmt.Errorf("proxy: %w", err)
		}
		if px.CheckedAt == "" {
			if px, err = s.CheckProxyByID(ctx, px.ID); err != nil {
				return err
			}
		}
		if px.Rotating {
			return apperr.Newf("proxy_rotating", "proxy %s changes its IP between connections; use a sticky port", px.Label)
		}
		if px.Timezone == "" {
			return apperr.Newf("proxy_no_timezone", "could not determine the timezone of proxy %s", px.Label)
		}
		p.Timezone, p.Lat, p.Lon, country = px.Timezone, px.Lat, px.Lon, px.Country
	}
	ui, langs, err := s.Locale(ctx, country)
	if err != nil {
		return err
	}
	p.UILocale, p.Languages = ui, strings.Join(langs, ",")
	return nil
}

func (s *Service) Trash(id string) error {
	if err := s.Launcher.Stop(id); err != nil {
		return err
	}
	if err := s.Store.SoftDelete(id); err != nil {
		return err
	}
	s.notify("profiles")
	return nil
}

func (s *Service) Restore(id string) error {
	if err := s.Store.Restore(id); err != nil {
		return err
	}
	s.notify("profiles")
	return nil
}

// Purge deletes a trashed profile for good: its folder, cookie key and row.
func (s *Service) Purge(id string) error {
	p, err := s.Store.GetProfile(id)
	if err != nil {
		return err
	}
	if p.DeletedAt == "" {
		return apperr.New("profile_not_trashed", "move the profile to the trash first")
	}
	if err := s.Launcher.Stop(id); err != nil {
		return err
	}
	if err := os.RemoveAll(s.Dirs.Profile(id)); err != nil {
		return err
	}
	os.Remove(filepath.Join(s.Dirs.Logs(), "browser-"+id+".log"))
	if err := s.Secrets.Delete(secrets.ServiceCookieKey, id); err != nil {
		return err
	}
	if err := s.Secrets.Delete(secrets.ServiceAccountPassword, id); err != nil {
		return err
	}
	if err := s.Secrets.Delete(secrets.ServiceTOTP, id); err != nil {
		return err
	}
	if err := s.Store.Purge(id); err != nil {
		return err
	}
	s.notify("profiles")
	return nil
}

// StartMany and StopMany run in parallel and report per-profile errors.
func (s *Service) StartMany(ctx context.Context, ids []string) map[string]string {
	return parallel(ids, func(id string) error { return s.Launcher.Start(ctx, id) })
}

func (s *Service) StopMany(ids []string) map[string]string {
	return parallel(ids, s.Launcher.Stop)
}

func parallel(ids []string, fn func(string) error) map[string]string {
	type res struct{ id, err string }
	ch := make(chan res, len(ids))
	for _, id := range ids {
		go func(id string) {
			r := res{id: id}
			if err := fn(id); err != nil {
				r.err = err.Error()
			}
			ch <- r
		}(id)
	}
	errs := map[string]string{}
	for range ids {
		if r := <-ch; r.err != "" {
			errs[r.id] = r.err
		}
	}
	return errs
}
