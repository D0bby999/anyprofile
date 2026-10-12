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
	return s.Store.SetSetting("language", lang)
}
