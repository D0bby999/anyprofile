// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/fingerprint"
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
	ProxyExpiry  string         `json:"proxyExpiry"` // "", "soon" or "expired"
	LastExit     *ExitSummary   `json:"lastExit"`    // nil until a launch measured one
	Flags        []Flag         `json:"flags"`
	// Checks are the consistency checks of the newest measured launch.
	Checks []fingerprint.Check `json:"checks"`
	// Position is the row's place in the order the user dragged the list
	// into, from 1; 0 for a profile not placed yet (after the placed ones).
	Position int `json:"position"`
}

// AutoProxy as NewProfile.ProxyID picks the next free sticky proxy (none if
// every proxy is taken).
const AutoProxy = "auto"

// NewProfile is what the create form sends. Identity is never chosen by hand.
type NewProfile struct {
	Engine   string   `json:"engine"` // "" means the default engine
	Name     string   `json:"name"`
	ProxyID  string   `json:"proxyId"`
	Sites    []string `json:"sites"`
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
	ids := make([]string, len(ps))
	for i, p := range ps {
		ids[i] = p.ID
	}
	exits, err := s.Store.LatestExits(ids)
	if err != nil {
		return nil, err
	}
	checks, err := s.Store.LatestChecks(ids)
	if err != nil {
		return nil, err
	}
	place := map[string]int{}
	for i, id := range s.profileOrder() {
		place[id] = i + 1
	}
	out := make([]ProfileRow, 0, len(ps))
	for _, p := range ps {
		row := ProfileRow{Profile: p, State: launcher.Stopped, Position: place[p.ID]}
		pair := exits[p.ID]
		row.Flags = exitFlags(p, &pair[0], &pair[1])
		row.Checks = []fingerprint.Check{}
		if raw, ok := checks[p.ID]; ok {
			_ = json.Unmarshal(raw, &row.Checks)
		}
		if pair[0].ExitIP != "" {
			row.LastExit = &ExitSummary{IP: pair[0].ExitIP, Country: pair[0].ExitCountry, At: pair[0].StartedAt}
		}
		if in, ok := running[p.ID]; ok {
			row.State, row.StateError, row.CDPPort = in.State, in.Error, in.CDPPort
		}
		if px, ok := byID[p.ProxyID]; ok {
			row.ProxyLabel, row.ProxyCountry, row.ProxyExitIP = px.Label, px.Country, px.ExitIP
			row.ProxyExpiry = expiry(px.ExpiresOn, s.now())
		}
		out = append(out, row)
	}
	return out, nil
}

// maxOrder bounds the stored order; far more profiles than anyone keeps.
const maxOrder = 100000

// profileOrder is the order the user dragged the profiles into; empty when
// never dragged, or unreadable.
func (s *Service) profileOrder() []string {
	v, _ := s.Store.GetSetting(store.SettingProfileOrder)
	var ids []string
	if v != "" {
		_ = json.Unmarshal([]byte(v), &ids)
	}
	return ids
}

// SetProfileOrder keeps the order the user dragged the profiles into. IDs
// of profiles gone since are dropped; a repeated ID keeps its first place.
func (s *Service) SetProfileOrder(ids []string) error {
	if len(ids) > maxOrder {
		return apperr.New("order_too_long", "too many profiles in the order")
	}
	ps, err := s.Store.ListProfiles(store.ProfileFilter{})
	if err != nil {
		return err
	}
	known := make(map[string]bool, len(ps))
	for _, p := range ps {
		known[p.ID] = true
	}
	keep := make([]string, 0, len(ids))
	for _, id := range ids {
		if known[id] {
			keep = append(keep, id)
			known[id] = false
		}
	}
	b, err := json.Marshal(keep)
	if err != nil {
		return err
	}
	if err := s.Store.SetSetting(store.SettingProfileOrder, string(b)); err != nil {
		return err
	}
	s.notify("profiles")
	return nil
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
	if err := s.checkSites(in.Sites); err != nil {
		return store.Profile{}, err
	}
	eng, err := s.engineFor(in.Engine)
	if err != nil {
		return store.Profile{}, err
	}
	if in.ProxyID == AutoProxy {
		id, err := s.nextFreeProxy()
		if err != nil {
			return store.Profile{}, err
		}
		in.ProxyID = id
	}
	id, err := eng.NewIdentity(ctx, randomU64())
	if err != nil {
		return store.Profile{}, err
	}
	startURL, err := normalizeStartURLs(in.StartURL)
	if err != nil {
		return store.Profile{}, err
	}
	p := store.Profile{
		ID: randomHex(8), Name: in.Name, Engine: eng.ID(), EngineVersion: s.newProfileVersion(ctx, eng),
		EngineMeta: id.Meta, PersonaID: id.PersonaID, SeedHex: randomHex(8), Platform: id.Platform, ProxyID: in.ProxyID,
		Sites: in.Sites, StartURL: startURL, Notes: in.Notes, Tags: in.Tags,
	}
	if err := s.applyNetwork(ctx, &p); err != nil {
		return store.Profile{}, err
	}
	s.regionStartURLs(&p)
	if err := s.Store.CreateProfile(&p); err != nil {
		return store.Profile{}, err
	}
	s.notify("profiles")
	return p, nil
}

// UpdateProfile saves editable fields; a new proxy re-derives timezone,
// position and languages so the profile keeps matching its exit.
func (s *Service) UpdateProfile(ctx context.Context, p store.Profile) error {
	if err := s.checkSites(p.Sites); err != nil {
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
		s.regionStartURLs(&p) // a new country brings its own seller center
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

// proxyCountry is the country the proxy exits in, "" without one.
func (s *Service) proxyCountry(proxyID string) string {
	if proxyID == "" {
		return ""
	}
	px, err := s.Store.GetProxy(proxyID)
	if err != nil {
		return ""
	}
	return px.Country
}

// regionStartURLs points p's platform start pages at its proxy's country.
func (s *Service) regionStartURLs(p *store.Profile) {
	if all, err := s.Sites(); err == nil {
		p.StartURL = regionStartURLs(p.StartURL, p.Sites, all, s.proxyCountry(p.ProxyID))
	}
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

// Purge deletes a trashed profile for good: its folder, cookie key, account
// secrets and row.
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
	if err := os.RemoveAll(s.Dirs.ProfileSnapshots(id)); err != nil {
		return err
	}
	if err := s.Secrets.Delete(secrets.ServiceCookieKey, id); err != nil {
		return err
	}
	for _, a := range p.Accounts {
		if err := s.deleteAccountSecrets(a.SecretKey); err != nil {
			return err
		}
	}
	if err := s.Store.Purge(id); err != nil {
		return err
	}
	s.notify("profiles")
	return nil
}

// StartMany and StopMany run in parallel and report per-profile errors.
// The browsers open behind the app, like a single Open.
func (s *Service) StartMany(ctx context.Context, ids []string) map[string]string {
	return parallel(ids, func(id string) error { return s.Launcher.StartBehind(ctx, id) })
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

// ResetFingerprint clears the saved baseline and verdict; the next launch
// becomes the new reference.
func (s *Service) ResetFingerprint(id string) error {
	if err := s.Store.ResetFingerprint(id); err != nil {
		return err
	}
	s.notify("profiles")
	return nil
}
