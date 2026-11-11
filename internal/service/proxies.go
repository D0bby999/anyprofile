// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"sort"
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
	// Assignable: free, sticky, its last check passed and it has not
	// expired. Only these are handed out as "the next free proxy".
	Assignable bool `json:"assignable"`
}

// ImportResult reports a pasted list: how many were added and which lines failed.
type ImportResult struct {
	Added  int                   `json:"added"`
	Failed []netproxy.ParsedLine `json:"failed"`
	// Unreachable lists the labels of added proxies whose first check failed.
	Unreachable []string `json:"unreachable"`
	// Rejected lists the subset of Unreachable whose upstream rejected authentication.
	Rejected []string `json:"rejected,omitempty"`
}

// ProxyInput is what the single-proxy dialog sends.
type ProxyInput struct {
	Label     string `json:"label"`
	Scheme    string `json:"scheme"` // "http" | "https" | "socks5"
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Note      string `json:"note"`
	ExpiresOn string `json:"expiresOn"` // "YYYY-MM-DD" or ""
}

// AddProxyResult returns the saved proxy row and any initial check failure code.
type AddProxyResult struct {
	Proxy    ProxyRow `json:"proxy"`
	CheckErr string   `json:"checkErr,omitempty"`
}

func (s *Service) proxyRowFrom(p store.Proxy, usedBy []string) ProxyRow {
	r := ProxyRow{Proxy: p, UsedBy: append([]string{}, usedBy...), Expiry: expiry(p.ExpiresOn, s.now())}
	r.Assignable = len(r.UsedBy) == 0 && !p.Rotating && p.ExitIP != "" && p.FailedAt == "" && r.Expiry != "expired"
	return r
}

func (s *Service) createProxy(px store.Proxy, password string) error {
	if password != "" {
		if err := s.Secrets.Set(secrets.ServiceProxy, px.ID, []byte(password)); err != nil {
			return err
		}
	}
	if err := s.Store.CreateProxy(&px); err != nil {
		s.Secrets.Delete(secrets.ServiceProxy, px.ID)
		return err
	}
	return nil
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
		out = append(out, s.proxyRowFrom(p, use[p.ID]))
	}
	return out, nil
}

// AddProxy validates and stores one proxy, stores its password in the Keychain,
// then runs the initial check. Unreachable/rejected proxies stay saved so they can be retried.
func (s *Service) AddProxy(ctx context.Context, in ProxyInput) (AddProxyResult, error) {
	scheme, ok := netproxy.NormalizeScheme(in.Scheme)
	if !ok {
		return AddProxyResult{}, apperr.New("proxy_scheme", "only http, https or socks5")
	}
	host := strings.TrimSpace(in.Host)
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	if host == "" || strings.ContainsAny(host, " \t\r\n/@") {
		return AddProxyResult{}, apperr.New("proxy_host", "type a valid proxy host")
	}
	if in.Port < 1 || in.Port > 65535 {
		return AddProxyResult{}, apperr.New("proxy_port", "port must be between 1 and 65535")
	}
	if len([]rune(in.Note)) > 500 {
		return AddProxyResult{}, apperr.New("note_too_long", "at most 500 characters")
	}
	if err := checkDate(in.ExpiresOn); err != nil {
		return AddProxyResult{}, err
	}
	label := strings.TrimSpace(in.Label)
	if label == "" {
		label = fmt.Sprintf("%s:%d", host, in.Port)
	}
	px := store.Proxy{
		ID:        randomHex(8),
		Label:     label,
		Scheme:    scheme,
		Host:      host,
		Port:      in.Port,
		Username:  strings.TrimSpace(in.Username),
		Note:      in.Note,
		ExpiresOn: in.ExpiresOn,
	}
	if err := s.createProxy(px, in.Password); err != nil {
		return AddProxyResult{}, err
	}
	var checkErr string
	if checked, err := s.CheckProxyByID(ctx, px.ID); err != nil {
		s.Log.Warn("proxy check", "proxy", px.ID, "err", err)
		if strings.HasPrefix(err.Error(), "anyprofile:proxy_rejected:") {
			checkErr = "proxy_rejected"
		} else {
			checkErr = "proxy_unreachable"
		}
		if latest, gerr := s.Store.GetProxy(px.ID); gerr == nil {
			px = latest
		}
	} else {
		px = checked
	}
	s.notify("proxies")
	return AddProxyResult{Proxy: s.proxyRowFrom(px, nil), CheckErr: checkErr}, nil
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
			Port: up.Port, Username: up.Username, Note: line.Note, ExpiresOn: expiresOn}
		if err := s.createProxy(px, up.Password); err != nil {
			return res, err
		}
		added = append(added, px.ID)
	}
	res.Added = len(added)
	failed, rejected := s.checkMany(ctx, added)
	for _, id := range failed {
		if px, err := s.Store.GetProxy(id); err == nil {
			res.Unreachable = append(res.Unreachable, px.Label)
			if rejected[id] {
				res.Rejected = append(res.Rejected, px.Label)
			}
		}
	}
	s.notify("proxies")
	return res, nil
}

// checkMany checks proxies eight at a time and returns the ids that failed.
func (s *Service) checkMany(ctx context.Context, ids []string) ([]string, map[string]bool) {
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failed []string
	rejected := map[string]bool{}
	for _, id := range ids {
		wg.Add(1)
		sem <- struct{}{}
		go func(id string) {
			defer func() { <-sem; wg.Done() }()
			if _, err := s.CheckProxyByID(ctx, id); err != nil {
				s.Log.Warn("proxy check", "proxy", id, "err", err)
				mu.Lock()
				failed = append(failed, id)
				if strings.HasPrefix(err.Error(), "anyprofile:proxy_rejected:") {
					rejected[id] = true
				}
				mu.Unlock()
			}
		}(id)
	}
	wg.Wait()
	sort.Strings(failed)
	return failed, rejected
}

// CheckProxyByID measures the exit through the proxy and stores it.
func (s *Service) CheckProxyByID(ctx context.Context, id string) (store.Proxy, error) {
	px, err := s.Store.GetProxy(id)
	if err != nil {
		return px, err
	}
	up, err := s.proxyUpstream(id)
	if err != nil {
		return px, err
	}
	e, err := s.CheckProxy(ctx, up)
	if err != nil {
		if ferr := s.Store.SaveProxyFailure(id); ferr == nil {
			s.notify("proxies")
		}
		if netproxy.IsAuthError(err) {
			return px, apperr.Newf("proxy_rejected", "%v", err)
		}
		// The network detail goes to the log; the user gets a sentence.
		return px, apperr.Newf("proxy_unreachable", "%v", err)
	}
	px.ExitIP, px.Country, px.Timezone, px.Lat, px.Lon, px.Rotating = e.IP, e.Country, e.Timezone, e.Lat, e.Lon, e.Rotating
	px.ExitIP6, px.Timezone6 = e.IPv6, e.IPv6Timezone
	if err := s.Store.SaveProxyCheck(px); err != nil {
		return px, err
	}
	s.notify("proxies")
	return s.Store.GetProxy(id)
}

// CheckAllProxies checks every proxy and returns how many did not answer.
func (s *Service) CheckAllProxies(ctx context.Context) (int, error) {
	ps, err := s.Store.ListProxies()
	if err != nil {
		return 0, err
	}
	ids := make([]string, len(ps))
	for i, p := range ps {
		ids[i] = p.ID
	}
	failed, _ := s.checkMany(ctx, ids)
	return len(failed), nil
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

// DeleteProxy refuses while a running profile or mobile device depends on it.
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
	if name, err := s.mobileUsesProxy(id); err != nil {
		return err
	} else if name != "" {
		return apperr.Newf("proxy_in_use", "mobile device %s uses this proxy; close it first", name)
	}
	if err := s.Store.DeleteProxy(id); err != nil {
		return err
	}
	if err := s.Secrets.Delete(secrets.ServiceProxy, id); err != nil {
		return err
	}
	s.notify("proxies")
	s.notify("profiles")
	s.notify("mobile")
	return nil
}

// nextFreeProxy is the oldest assignable proxy, or "" for none.
func (s *Service) nextFreeProxy() (string, error) {
	rows, err := s.ListProxies()
	if err != nil {
		return "", err
	}
	for _, r := range rows {
		if r.Assignable {
			return r.ID, nil
		}
	}
	return "", nil
}
