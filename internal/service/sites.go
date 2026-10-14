// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/site"
	"github.com/D0bby999/anyprofile/internal/store"
)

// SiteInput is what the custom-platform form sends.
type SiteInput struct {
	Name     string   `json:"name"`
	Domains  []string `json:"domains"`
	StartURL string   `json:"startUrl"`
	Color    string   `json:"color"`
}

var (
	domainRe = regexp.MustCompile(`^[a-z0-9-]+(\.[a-z0-9-]+)+$`)
	colorRe  = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
)

// Sites returns the built-in catalog followed by the user's custom platforms.
func (s *Service) Sites() ([]site.Site, error) {
	out := site.Builtin()
	custom, err := s.Store.ListCustomSites()
	if err != nil {
		return nil, err
	}
	for _, c := range custom {
		out = append(out, site.Site{
			ID: c.ID, Name: c.Name, Group: "custom", Domains: c.Domains,
			StartURL: c.StartURL, Icon: "", Color: c.Color, Custom: true,
		})
	}
	return out, nil
}

// checkSite accepts "" (the store defaults it to other), a known site id, and
// nothing else.
func (s *Service) checkSite(id string) error {
	if id == "" {
		return nil
	}
	all, err := s.Sites()
	if err != nil {
		return err
	}
	for _, x := range all {
		if x.ID == id {
			return nil
		}
	}
	return apperr.Newf("unknown_site", "unknown platform %q", id)
}

// maxSites caps a profile's platforms; each one usually adds a start page,
// and a profile opens at most 10.
const maxSites = 10

// checkSites validates a profile's platforms; the store drops repeats and
// "other" beside a real platform.
func (s *Service) checkSites(ids []string) error {
	if len(ids) > maxSites {
		return apperr.Newf("too_many_sites", "at most %d platforms", maxSites)
	}
	for _, id := range ids {
		if err := s.checkSite(id); err != nil {
			return err
		}
	}
	return nil
}

// normalizeSite cleans and validates the custom-platform form.
func normalizeSite(in SiteInput) (SiteInput, error) {
	out := SiteInput{}
	out.Name = strings.TrimSpace(in.Name)
	if n := len([]rune(out.Name)); n < 1 || n > 40 {
		return out, apperr.New("site_name", "a platform name of 1-40 characters is required")
	}
	for _, d := range in.Domains {
		d = strings.ToLower(strings.TrimSpace(d))
		d = strings.TrimPrefix(d, "https://")
		d = strings.TrimPrefix(d, "http://")
		d = strings.TrimPrefix(d, "www.")
		if i := strings.IndexByte(d, '/'); i >= 0 {
			d = d[:i]
		}
		if d == "" {
			continue
		}
		if !domainRe.MatchString(d) {
			return out, apperr.Newf("site_domain", "%q is not a domain", d)
		}
		out.Domains = append(out.Domains, d)
	}
	if out.Domains == nil {
		out.Domains = []string{}
	}
	out.StartURL = strings.TrimSpace(in.StartURL)
	if out.StartURL != "" {
		u, err := url.Parse(out.StartURL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return out, apperr.New("site_url", "the start page must be an http(s) URL")
		}
	}
	if !colorRe.MatchString(in.Color) {
		return out, apperr.New("site_color", "pick a color")
	}
	out.Color = "#" + strings.ToUpper(in.Color[1:])
	return out, nil
}

// CreateSite adds a custom platform and returns it as a catalog entry.
func (s *Service) CreateSite(in SiteInput) (site.Site, error) {
	n, err := normalizeSite(in)
	if err != nil {
		return site.Site{}, err
	}
	c := store.CustomSite{ID: "custom-" + randomHex(4), Name: n.Name, Domains: n.Domains,
		StartURL: n.StartURL, Color: n.Color}
	if err := s.Store.CreateCustomSite(&c); err != nil {
		return site.Site{}, err
	}
	s.notify("sites")
	return site.Site{ID: c.ID, Name: c.Name, Group: "custom", Domains: c.Domains,
		StartURL: c.StartURL, Color: c.Color, Custom: true}, nil
}

// UpdateSite edits a custom platform; built-ins are read-only.
func (s *Service) UpdateSite(id string, in SiteInput) error {
	if !strings.HasPrefix(id, "custom-") {
		return apperr.New("site_builtin", "built-in platforms cannot be changed")
	}
	n, err := normalizeSite(in)
	if err != nil {
		return err
	}
	if err := s.Store.UpdateCustomSite(store.CustomSite{ID: id, Name: n.Name,
		Domains: n.Domains, StartURL: n.StartURL, Color: n.Color}); err != nil {
		return err
	}
	s.notify("sites")
	return nil
}

// DeleteSite removes a custom platform; its profiles move to other.
func (s *Service) DeleteSite(id string) error {
	if !strings.HasPrefix(id, "custom-") {
		return apperr.New("site_builtin", "built-in platforms cannot be changed")
	}
	if err := s.Store.DeleteCustomSite(id); err != nil {
		return err
	}
	s.notify("sites")
	s.notify("profiles")
	return nil
}
