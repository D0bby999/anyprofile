// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

// DefaultAPIPort is where the local API listens unless the user picks another.
const DefaultAPIPort = 51525

// APISettings is what Settings shows about the local API. The token is never
// part of it: it reaches the clipboard from Go only.
type APISettings struct {
	Enabled bool `json:"enabled"`
	Port    int  `json:"port"`
}

func (s *Service) APISettings() APISettings {
	a := APISettings{Port: DefaultAPIPort}
	if v, _ := s.Store.GetSetting("api_enabled"); v == "1" {
		a.Enabled = true
	}
	if v, _ := s.Store.GetSetting("api_port"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			a.Port = n
		}
	}
	return a
}

func (s *Service) SetAPISettings(a APISettings) error {
	if a.Port < 1024 || a.Port > 65535 {
		return apperr.New("api_bad_port", "the port must be between 1024 and 65535")
	}
	if a.Enabled {
		if _, err := s.APIToken(); err != nil { // make the token before anything can need it
			return err
		}
	}
	enabled := "0"
	if a.Enabled {
		enabled = "1"
	}
	if err := s.Store.SetSetting("api_enabled", enabled); err != nil {
		return err
	}
	if err := s.Store.SetSetting("api_port", strconv.Itoa(a.Port)); err != nil {
		return err
	}
	s.notify("settings")
	return nil
}

// APIToken returns the token, creating one on first use.
func (s *Service) APIToken() (string, error) {
	b, err := s.Secrets.Get(secrets.ServiceAPIToken, "app")
	if errors.Is(err, secrets.ErrNotFound) {
		return s.NewAPIToken()
	}
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// NewAPIToken replaces the token; scripts holding the old one stop working.
func (s *Service) NewAPIToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(b)
	if err := s.Secrets.Set(secrets.ServiceAPIToken, "app", []byte(tok)); err != nil {
		return "", err
	}
	return tok, nil
}

// StartProfile opens a profile for a script. With devtools, this launch gets
// a loopback DevTools port; a profile already running without one is refused
// rather than restarted behind the user's back.
func (s *Service) StartProfile(ctx context.Context, id string, devtools bool) error {
	p, err := s.Store.GetProfile(id)
	if err != nil {
		return err
	}
	if p.DeletedAt != "" {
		return apperr.New("profile_trashed", "move the profile out of the trash first")
	}
	if in, running := s.Launcher.Info(id); running {
		if devtools && in.CDPPort == 0 {
			return apperr.New("api_running_without_devtools", "the profile is open without DevTools; stop it first")
		}
		return nil
	}
	if devtools && !p.CDPNextLaunch {
		p.CDPNextLaunch = true
		if err := s.Store.UpdateProfile(p); err != nil {
			return err
		}
	}
	return s.Launcher.StartBehind(ctx, id)
}

// ProfileState is the launch state of one profile.
func (s *Service) ProfileState(id string) launcher.Info {
	if in, ok := s.Launcher.Info(id); ok {
		return in
	}
	return launcher.Info{ProfileID: id, State: launcher.Stopped}
}

// LiveProfiles lists the profiles not in the trash.
func (s *Service) LiveProfiles() ([]ProfileRow, error) {
	return s.ListProfiles(store.ProfileFilter{})
}
