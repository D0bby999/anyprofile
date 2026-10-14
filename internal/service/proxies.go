// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/netproxy"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

// ProxyRow is a proxy plus the live profiles that use it. No password, ever.
type ProxyRow struct {
	store.Proxy
	UsedBy []string `json:"usedBy"`
	Expiry string   `json:"expiry"` // "", "soon" or "expired"
}

// ImportResult reports a pasted list: how many were added and which lines failed.
type ImportResult struct {
	Added  int                   `json:"added"`
	Failed []netproxy.ParsedLine `json:"failed"`
}

func (s *Service) ListProxies() ([]ProxyRow, error) {
	ps, err := s.Store.ListProxies()
	if err != nil {
		return nil, err
	}
	use, err := s.Store.ProxyUsage()
	if err != nil {
		return nil, err
	}
	out := make([]ProxyRow, 0, len(ps))
	for _, p := range ps {
		out = append(out, ProxyRow{Proxy: p, UsedBy: append([]string{}, use[p.ID]...),
			Expiry: expiry(p.ExpiresOn, s.now())})
	}
	return out, nil
}

// ImportProxies stores each valid line (password into the Keychain) with the
// same expiry date, then checks the new proxies in parallel.
func (s *Service) ImportProxies(ctx context.Context, text, expiresOn string) (ImportResult, error) {
	if err := checkDate(expiresOn); err != nil {
		return ImportResult{}, err
	}
	var res ImportResult
	var added []string
	for _, line := range netproxy.ParseList(text) {
		if line.Err != "" {
			line.Upstream = netproxy.Upstream{} // never hand a password back to the UI
			res.Failed = append(res.Failed, line)
			continue
		}
		up := line.Upstream
		px := store.Proxy{ID: randomHex(8), Label: fmt.Sprintf("%s:%d", up.Host, up.Port), Scheme: up.Scheme, Host: up.Host,
			Port: up.Port, Username: up.Username, ExpiresOn: expiresOn}
		if up.Password != "" {
			if err := s.Secrets.Set(secrets.ServiceProxy, px.ID, []byte(up.Password)); err != nil {
				return res, err
			}
		}
		if err := s.Store.CreateProxy(&px); err != nil {
			s.Secrets.Delete(secrets.ServiceProxy, px.ID)
			return res, err
		}
		added = append(added, px.ID)
	}
	res.Added = len(added)
	s.checkMany(ctx, added)
	s.notify("proxies")
	return res, nil
}

func (s *Service) checkMany(ctx context.Context, ids []string) {
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		sem <- struct{}{}
		go func(id string) {
			defer func() { <-sem; wg.Done() }()
			if _, err := s.CheckProxyByID(ctx, id); err != nil {
				s.Log.Warn("proxy check", "proxy", id, "err", err)
			}
		}(id)
	}
	wg.Wait()
}

// CheckProxyByID measures the exit through the proxy and stores it.
func (s *Service) CheckProxyByID(ctx context.Context, id string) (store.Proxy, error) {
	px, err := s.Store.GetProxy(id)
	if err != nil {
		return px, err
	}
	up := netproxy.Upstream{Scheme: px.Scheme, Host: px.Host, Port: px.Port, Username: px.Username}
	if pw, err := s.Secrets.Get(secrets.ServiceProxy, id); err == nil {
		up.Password = string(pw)
	} else if !errors.Is(err, secrets.ErrNotFound) {
		return px, err
	}
	e, err := s.CheckProxy(ctx, up)
	if err != nil {
		// The network detail goes to the log; the user gets a sentence.
		return px, apperr.Newf("proxy_unreachable", "%v", err)
	}
	px.ExitIP, px.Country, px.Timezone, px.Lat, px.Lon, px.Rotating = e.IP, e.Country, e.Timezone, e.Lat, e.Lon, e.Rotating
	if err := s.Store.SaveProxyCheck(px); err != nil {
		return px, err
	}
	s.notify("proxies")
	return s.Store.GetProxy(id)
}

func (s *Service) CheckAllProxies(ctx context.Context) error {
	ps, err := s.Store.ListProxies()
	if err != nil {
		return err
	}
	ids := make([]string, len(ps))
	for i, p := range ps {
		ids[i] = p.ID
	}
	s.checkMany(ctx, ids)
	return nil
}

// ProxyMeta is what the proxy edit form sends.
type ProxyMeta struct {
	Label     string `json:"label"`
	Note      string `json:"note"`
	ExpiresOn string `json:"expiresOn"` // "YYYY-MM-DD" or ""
}

// expiry is "", "soon" (3 days or fewer left) or "expired", seen from now.
func expiry(expiresOn string, now time.Time) string {
	if expiresOn == "" {
		return ""
	}
	d, err := time.ParseInLocation("2006-01-02", expiresOn, now.Location())
	if err != nil {
		return ""
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if d.Before(today) {
		return "expired"
	}
	if d.Sub(today) <= 72*time.Hour {
		return "soon"
	}
	return ""
}

func checkDate(s string) error {
	if s == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return apperr.New("bad_date", "use a date like 2026-12-31")
	}
	return nil
}

// UpdateProxy saves the proxy's label, note and expiry date.
func (s *Service) UpdateProxy(id string, m ProxyMeta) error {
	m.Label = strings.TrimSpace(m.Label)
	if m.Label == "" {
		return apperr.New("proxy_label", "type a name")
	}
	if len([]rune(m.Note)) > 500 {
		return apperr.New("note_too_long", "at most 500 characters")
	}
	if err := checkDate(m.ExpiresOn); err != nil {
		return err
	}
	if err := s.Store.UpdateProxyMeta(id, m.Label, m.Note, m.ExpiresOn); err != nil {
		return err
	}
	s.notify("proxies")
	return nil
}

// DeleteProxy refuses while a running profile depends on it.
func (s *Service) DeleteProxy(id string) error {
	running := map[string]bool{}
	for _, in := range s.Launcher.List() {
		if in.State != launcher.Stopped {
			running[in.ProfileID] = true
		}
	}
	ps, err := s.Store.ListProfiles(store.ProfileFilter{})
	if err != nil {
		return err
	}
	for _, p := range ps {
		if p.ProxyID == id && running[p.ID] {
			return apperr.New("proxy_in_use", "a running profile uses this proxy; close it first")
		}
	}
	if err := s.Store.DeleteProxy(id); err != nil {
		return err
	}
	if err := s.Secrets.Delete(secrets.ServiceProxy, id); err != nil {
		return err
	}
	s.notify("proxies")
	s.notify("profiles")
	return nil
}

// nextFreeProxy is the oldest sticky proxy no live profile uses, or "" for none.
func (s *Service) nextFreeProxy() (string, error) {
	rows, err := s.ListProxies()
	if err != nil {
		return "", err
	}
	for _, r := range rows {
		if len(r.UsedBy) == 0 && !r.Rotating {
			return r.ID, nil
		}
	}
	return "", nil
}
