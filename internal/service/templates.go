// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/text/language"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/store"
)

// Built-in templates ship with the app. They are code, not rows: the
// user can copy one into an editable template of their own, but the
// presets themselves never change and cannot be deleted. Languages are
// home-market defaults, not a claim that every proxy exits in that market.
// Explicit template languages are applied at creation; launch checks report
// inconsistencies but do not silently rewrite a user's language choice.
var builtinTemplates = []store.Template{
	{
		ID: "builtin-tiktok", Name: "TikTok", Site: "tiktok",
		Sites: []string{"tiktok"}, Builtin: true,
		ProxyNote: "Proxy dân cư Việt Nam, IP tĩnh (sticky)",
		Languages: "vi-VN,vi", UILocale: "vi-VN",
		StartURL: "https://www.tiktok.com/", Tags: []string{"tiktok"},
	},
	{
		ID: "builtin-youtube", Name: "YouTube", Site: "youtube",
		Sites: []string{"youtube"}, Builtin: true,
		ProxyNote: "Proxy dân cư, IP tĩnh (sticky)",
		Languages: "vi-VN,vi", UILocale: "vi-VN",
		StartURL: "https://www.youtube.com/", Tags: []string{"youtube"},
	},
	{
		ID: "builtin-facebook", Name: "Facebook", Site: "facebook",
		Sites: []string{"facebook"}, Builtin: true,
		ProxyNote: "Proxy dân cư Việt Nam, IP tĩnh (sticky)",
		Languages: "vi-VN,vi", UILocale: "vi-VN",
		StartURL: "https://www.facebook.com/", Tags: []string{"facebook"},
	},
	{
		ID: "builtin-blank", Name: "Trống", Site: "other",
		Sites: []string{"other"}, Builtin: true,
	},
}

// ListTemplates returns the built-in presets followed by the user's own
// templates.
func (s *Service) ListTemplates() ([]store.Template, error) {
	user, err := s.Store.ListTemplates()
	if err != nil {
		return nil, err
	}
	out := make([]store.Template, 0, len(builtinTemplates)+len(user))
	out = append(out, builtinTemplates...)
	out = append(out, user...)
	return out, nil
}

// templateByID resolves a built-in or user template id.
func (s *Service) templateByID(id string) (store.Template, error) {
	for _, t := range builtinTemplates {
		if t.ID == id {
			return t, nil
		}
	}
	return s.Store.GetTemplate(id)
}

// CreateTemplate stores a new user template. Copying a built-in is just
// creating a user template with its fields.
func (s *Service) CreateTemplate(t store.Template) (store.Template, error) {
	t.ID = randomHex(8)
	t.Builtin = false
	if err := s.validateTemplate(&t); err != nil {
		return store.Template{}, err
	}
	if err := s.Store.CreateTemplate(&t); err != nil {
		return store.Template{}, err
	}
	s.notify("settings")
	return t, nil
}

// UpdateTemplate edits a user template; built-ins are read-only.
func (s *Service) UpdateTemplate(t store.Template) error {
	if strings.HasPrefix(t.ID, "builtin-") || t.Builtin {
		return apperr.New("builtin_template", "built-in templates are read-only; duplicate it to edit a copy")
	}
	if err := s.validateTemplate(&t); err != nil {
		return err
	}
	if err := s.Store.UpdateTemplate(&t); err != nil {
		return err
	}
	s.notify("settings")
	return nil
}

// DeleteTemplate removes a user template; built-ins cannot be deleted.
func (s *Service) DeleteTemplate(id string) error {
	for _, t := range builtinTemplates {
		if t.ID == id {
			return apperr.New("builtin_template", "built-in templates cannot be deleted")
		}
	}
	if err := s.Store.DeleteTemplate(id); err != nil {
		return err
	}
	s.notify("settings")
	return nil
}

// DuplicateTemplate copies any template (built-in or user) into a new
// user template.
func (s *Service) DuplicateTemplate(id string) (store.Template, error) {
	src, err := s.templateByID(id)
	if err != nil {
		return store.Template{}, err
	}
	src.Name += " (bản sao)"
	return s.CreateTemplate(src)
}

func (s *Service) validateTemplate(t *store.Template) error {
	if len(t.Sites) == 0 && t.Site != "" {
		t.Sites = []string{t.Site}
	}
	if err := s.checkSites(t.Sites); err != nil {
		return err
	}
	if len(t.Sites) > 0 {
		t.Site = t.Sites[0]
	}
	if t.Engine != "" {
		if _, err := s.engineFor(t.Engine); err != nil {
			return err
		}
	}
	urls, err := normalizeStartURLs(t.StartURL)
	if err != nil {
		return err
	}
	t.StartURL = urls
	if t.FolderID != "" {
		if _, err := s.Store.GetFolder(t.FolderID); err != nil {
			return err
		}
	}
	if t.Languages != "" {
		for _, lang := range strings.Split(t.Languages, ",") {
			if !validTemplateLanguage(strings.TrimSpace(lang)) {
				return apperr.New("bad_template_language", "template languages must be comma-separated BCP 47 language tags")
			}
		}
	}
	if t.UILocale != "" {
		if !validTemplateLanguage(t.UILocale) {
			return apperr.New("bad_template_language", "template UI locale must be a BCP 47 language tag")
		}
	}
	return nil
}

// language.Parse also accepts locale syntax such as en_US; templates promise
// BCP 47 tags, so reject non-tag characters before parsing.
func validTemplateLanguage(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	_, err := language.Parse(s)
	return err == nil
}

// CreateProfileFromTemplate creates a profile with the template's
// defaults: its sites, start page, notes, tags, engine and folder. The
// template's language becomes the profile's language; the proxy is the
// caller's choice (AutoProxy, a proxy id, or ""), because a template
// only recommends a proxy kind, it never carries one.
func (s *Service) CreateProfileFromTemplate(ctx context.Context, templateID, name, proxyID string) (store.Profile, error) {
	t, err := s.templateByID(templateID)
	if err != nil {
		return store.Profile{}, err
	}
	sites := t.Sites
	if len(sites) == 0 && t.Site != "" {
		sites = []string{t.Site}
	}
	return s.createProfile(ctx, NewProfile{
		Engine: t.Engine, Name: name, ProxyID: proxyID,
		Sites: sites, StartURL: t.StartURL, Notes: t.Notes, Tags: t.Tags,
	}, func(p *store.Profile) error {
		if t.Languages != "" {
			p.Languages = t.Languages
		}
		if t.UILocale != "" {
			p.UILocale = t.UILocale
		}
		if t.FolderID != "" {
			if _, err := s.Store.GetFolder(t.FolderID); err == nil {
				p.FolderID = t.FolderID
			} else if !errors.Is(err, store.ErrNotFound) {
				return err
			}
		}
		return nil
	})
}
