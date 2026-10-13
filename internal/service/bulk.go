// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strconv"
	"strings"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/store"
)

// nameSeries names n new profiles: "base" alone, or "base 1" .. "base n".
func nameSeries(base string, n int) []string {
	if n == 1 {
		return []string{base}
	}
	out := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, base+" "+strconv.Itoa(i))
	}
	return out
}

// BulkCreate is the "create several" form.
type BulkCreate struct {
	BaseName string   `json:"baseName"`
	Count    int      `json:"count"` // 1..100
	Proxy    string   `json:"proxy"` // "auto" (one free proxy each) or "none"
	Site     string   `json:"site"`
	StartURL string   `json:"startUrl"`
	Notes    string   `json:"notes"`
	Tags     []string `json:"tags"`
}

// DuplicateInput copies a profile's settings, never its identity or account.
type DuplicateInput struct {
	SourceID string `json:"sourceId"`
	BaseName string `json:"baseName"` // "" means "<source name> copy"
	Count    int    `json:"count"`    // 1..50
	Proxy    string `json:"proxy"`    // "auto" or "none"
}

func (s *Service) freeProxyCount() (int, error) {
	rows, err := s.ListProxies()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range rows {
		if len(r.UsedBy) == 0 && !r.Rotating {
			n++
		}
	}
	return n, nil
}

func checkProxyMode(mode string) error {
	if mode != "auto" && mode != "none" {
		return apperr.New("bad_proxy_mode", "proxy must be auto or none")
	}
	return nil
}

// CreateProfiles makes count profiles, each with its own free proxy or none.
func (s *Service) CreateProfiles(ctx context.Context, in BulkCreate) ([]store.Profile, error) {
	if in.Count < 1 || in.Count > 100 {
		return nil, apperr.New("bad_count", "between 1 and 100 profiles")
	}
	in.BaseName = strings.TrimSpace(in.BaseName)
	if in.BaseName == "" {
		return nil, apperr.New("name_required", "profile name is required")
	}
	if err := checkProxyMode(in.Proxy); err != nil {
		return nil, err
	}
	if err := s.checkSite(in.Site); err != nil {
		return nil, err
	}
	if in.Proxy == "auto" {
		free, err := s.freeProxyCount()
		if err != nil {
			return nil, err
		}
		if free < in.Count {
			return nil, apperr.Newf("not_enough_proxies", "need %d free sticky proxies, have %d", in.Count, free)
		}
	}
	out := make([]store.Profile, 0, in.Count)
	for _, name := range nameSeries(in.BaseName, in.Count) {
		proxyID := ""
		if in.Proxy == "auto" {
			proxyID = AutoProxy
		}
		p, err := s.CreateProfile(ctx, NewProfile{Name: name, ProxyID: proxyID, Site: in.Site,
			StartURL: in.StartURL, Notes: in.Notes, Tags: in.Tags})
		if err != nil {
			return out, err
		}
		out = append(out, p)
	}
	return out, nil
}

// DuplicateProfile copies a profile's platform, start pages, tags and notes
// into count new profiles with fresh identities. Accounts and cookies are
// never copied.
func (s *Service) DuplicateProfile(ctx context.Context, in DuplicateInput) ([]store.Profile, error) {
	src, err := s.Store.GetProfile(in.SourceID)
	if err != nil {
		return nil, err
	}
	if src.DeletedAt != "" {
		return nil, apperr.New("profile_trashed", "move the profile out of the trash first")
	}
	if in.Count < 1 || in.Count > 50 {
		return nil, apperr.New("bad_count", "between 1 and 50 profiles")
	}
	if err := checkProxyMode(in.Proxy); err != nil {
		return nil, err
	}
	base := in.BaseName
	if base == "" {
		base = src.Name + " copy"
	}
	tags := append([]string(nil), src.Tags...)
	return s.CreateProfiles(ctx, BulkCreate{BaseName: base, Count: in.Count, Proxy: in.Proxy,
		Site: src.Site, StartURL: src.StartURL, Notes: src.Notes, Tags: tags})
}

// SetSiteMany assigns one platform to several profiles.
func (s *Service) SetSiteMany(ids []string, siteID string) error {
	if err := s.checkSite(siteID); err != nil {
		return err
	}
	for _, id := range ids {
		p, err := s.Store.GetProfile(id)
		if err != nil {
			return err
		}
		p.Site = siteID
		if err := s.Store.UpdateProfile(p); err != nil {
			return err
		}
	}
	s.notify("profiles")
	return nil
}

// AddTagMany adds one tag to several profiles, without duplicating it.
func (s *Service) AddTagMany(ids []string, tag string) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return apperr.New("tag_empty", "type a tag")
	}
	for _, id := range ids {
		p, err := s.Store.GetProfile(id)
		if err != nil {
			return err
		}
		found := false
		for _, x := range p.Tags {
			if x == tag {
				found = true
				break
			}
		}
		if !found {
			p.Tags = append(p.Tags, tag)
			if err := s.Store.UpdateProfile(p); err != nil {
				return err
			}
		}
	}
	s.notify("profiles")
	return nil
}

// AssignProxiesMany assigns a proxy to each profile (sequentially, so the
// auto picker sees each previous assignment) and reports per-profile errors.
func (s *Service) AssignProxiesMany(ctx context.Context, ids []string, mode string) map[string]string {
	out := map[string]string{}
	if err := checkProxyMode(mode); err != nil {
		for _, id := range ids {
			out[id] = err.Error()
		}
		return out
	}
	running := map[string]bool{}
	for _, in := range s.Launcher.List() {
		if in.State != launcher.Stopped {
			running[in.ProfileID] = true
		}
	}
	for _, id := range ids {
		if running[id] {
			out[id] = apperr.New("profile_running", "close the profile first").Error()
			continue
		}
		p, err := s.Store.GetProfile(id)
		if err != nil {
			out[id] = err.Error()
			continue
		}
		if mode == "none" {
			p.ProxyID = ""
		} else {
			next, err := s.nextFreeProxy()
			if err != nil {
				out[id] = err.Error()
				continue
			}
			if next == "" {
				out[id] = apperr.New("not_enough_proxies", "no free sticky proxy left").Error()
				continue
			}
			p.ProxyID = next
		}
		if err := s.UpdateProfile(ctx, p); err != nil {
			out[id] = err.Error()
		}
	}
	s.notify("profiles")
	return out
}
