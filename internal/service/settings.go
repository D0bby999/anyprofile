// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"sort"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/store"
)

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

// DefaultEngine is the engine for new profiles: the saved choice when it is
// registered, else Preferred, else any registered engine (sorted by ID).
func (s *Service) DefaultEngine() string {
	if v, err := s.Store.GetSetting("default_engine"); err == nil && s.Engines[v] != nil {
		return v
	}
	if s.Engines[s.Preferred] != nil {
		return s.Preferred
	}
	ids := make([]string, 0, len(s.Engines))
	for id := range s.Engines {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

// engineFor resolves an engine ID; "" means the default engine.
func (s *Service) engineFor(id string) (engine.Engine, error) {
	if id == "" {
		id = s.DefaultEngine()
	}
	if eng := s.Engines[id]; eng != nil {
		return eng, nil
	}
	return nil, apperr.New("bad_engine", "unknown engine")
}

// Defaults are the new-profile defaults (engine, platform, proxy mode) and
// what every profile opens with.
type Defaults struct {
	Engine string `json:"engine"`
	Site   string `json:"site"`   // default "other"
	Proxy  string `json:"proxy"`  // "auto" (default) or "none"
	OnOpen string `json:"onOpen"` // "restore" (default) or "start"
}

// Defaults reads the saved defaults, falling back to other/auto/restore.
func (s *Service) Defaults() Defaults {
	d := Defaults{Engine: s.DefaultEngine(), Site: "other", Proxy: "auto", OnOpen: "restore"}
	if v, err := s.Store.GetSetting("default_site"); err == nil && v != "" && s.checkSite(v) == nil {
		d.Site = v
	}
	if v, err := s.Store.GetSetting("default_proxy"); err == nil && v == "none" {
		d.Proxy = "none"
	}
	if v, err := s.Store.GetSetting(store.SettingOnOpen); err == nil && v == "start" {
		d.OnOpen = "start"
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
	if d.OnOpen != "restore" && d.OnOpen != "start" {
		return apperr.New("bad_on_open", "on open must be restore or start")
	}
	if s.Engines[d.Engine] == nil {
		return apperr.New("bad_engine", "unknown engine")
	}
	if err := s.Store.SetSetting("default_engine", d.Engine); err != nil {
		return err
	}
	if err := s.Store.SetSetting("default_site", d.Site); err != nil {
		return err
	}
	if err := s.Store.SetSetting("default_proxy", d.Proxy); err != nil {
		return err
	}
	if err := s.Store.SetSetting(store.SettingOnOpen, d.OnOpen); err != nil {
		return err
	}
	s.notify("settings")
	return nil
}

// Onboarded reports whether first-run setup is done. Someone who already has
// profiles (from before setup existed) counts as done, and that is saved.
func (s *Service) Onboarded() bool {
	if v, err := s.Store.GetSetting("onboarded"); err == nil && v == "1" {
		return true
	}
	if n, err := s.Store.ProfileCount(); err == nil && n > 0 {
		if err := s.Store.SetSetting("onboarded", "1"); err != nil {
			s.Log.Warn("save onboarded", "err", err)
		}
		return true
	}
	return false
}

// FinishOnboarding marks first-run setup as done (finished or skipped).
func (s *Service) FinishOnboarding() error {
	if err := s.Store.SetSetting("onboarded", "1"); err != nil {
		return err
	}
	s.notify("settings")
	return nil
}
