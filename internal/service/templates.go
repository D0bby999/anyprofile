// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/store"
)

// Built-in templates ship with the app. They are code, not rows: the
// user can copy one into an editable template of their own, but the
// presets themselves never change and cannot be deleted. Languages are
// the app's home-market defaults; the launch consistency checks still
// re-derive locale from the assigned proxy, so a template can suggest a
// language but never forces a mismatch past the checks.
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
	if err := s.checkTemplateSites(t); err != nil {
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
	if t.Builtin {
		return apperr.New("builtin_template", "built-in templates are read-only; duplicate it to edit a copy")
	}
	if err := s.checkTemplateSites(t); err != nil {
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

func (s *Service) checkTemplateSites(t store.Template) error {
	if len(t.Sites) > 0 {
		return s.checkSites(t.Sites)
	}
	if t.Site != "" {
		return s.checkSites([]string{t.Site})
	}
	return nil
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
	p, err := s.CreateProfile(ctx, NewProfile{
		Engine: t.Engine, Name: name, ProxyID: proxyID,
		Sites: sites, StartURL: t.StartURL, Notes: t.Notes, Tags: t.Tags,
	})
	if err != nil {
		return store.Profile{}, err
	}
	if t.Languages != "" || t.UILocale != "" {
		if t.Languages != "" {
			p.Languages = t.Languages
		}
		if t.UILocale != "" {
			p.UILocale = t.UILocale
		}
		if err := s.Store.UpdateProfile(p); err != nil {
			return store.Profile{}, err
		}
	}
	if t.FolderID != "" {
		if _, err := s.Store.GetFolder(t.FolderID); err == nil {
			if err := s.Store.SetProfileFolder(p.ID, t.FolderID); err != nil {
				return store.Profile{}, err
			}
			p.FolderID = t.FolderID
		}
	}
	return p, nil
}
