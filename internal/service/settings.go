// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import "github.com/D0bby999/anyprofile/internal/apperr"

// Languages the UI ships. English is the default for a new install.
var languages = map[string]bool{"en": true, "vi": true}

// Language returns the persisted UI language, defaulting to English.
func (s *Service) Language() string {
	if v, err := s.Store.GetSetting("language"); err == nil && languages[v] {
		return v
	}
	return "en"
}

// SetLanguage persists the UI language choice.
func (s *Service) SetLanguage(lang string) error {
	if !languages[lang] {
		return apperr.Newf("bad_language", "unsupported language %q", lang)
	}
	if err := s.Store.SetSetting("language", lang); err != nil {
		return err
	}
	s.notify("settings")
	return nil
}

// Defaults are the new-profile defaults: the platform and the proxy mode.
type Defaults struct {
	Site  string `json:"site"`  // default "other"
	Proxy string `json:"proxy"` // "auto" (default) or "none"
}

// Defaults reads the saved new-profile defaults, falling back to other/auto.
func (s *Service) Defaults() Defaults {
	d := Defaults{Site: "other", Proxy: "auto"}
	if v, err := s.Store.GetSetting("default_site"); err == nil && v != "" && s.checkSite(v) == nil {
		d.Site = v
	}
	if v, err := s.Store.GetSetting("default_proxy"); err == nil && v == "none" {
		d.Proxy = "none"
	}
	return d
}

// SetDefaults saves the new-profile defaults.
func (s *Service) SetDefaults(d Defaults) error {
	if err := s.checkSite(d.Site); err != nil {
		return err
	}
	if d.Proxy != "auto" && d.Proxy != "none" {
		return apperr.New("bad_proxy_mode", "proxy must be auto or none")
	}
	if err := s.Store.SetSetting("default_site", d.Site); err != nil {
		return err
	}
	if err := s.Store.SetSetting("default_proxy", d.Proxy); err != nil {
		return err
	}
	s.notify("settings")
	return nil
}
