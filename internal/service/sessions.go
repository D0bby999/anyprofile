// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"github.com/D0bby999/anyprofile/internal/netproxy"
	"github.com/D0bby999/anyprofile/internal/store"
)

// Flag is a warning about a profile's exit; the UI turns ID and Params into
// a sentence, so no English is built here.
type Flag struct {
	ID     string            `json:"id"`    // country_changed, ip_changed, tz_mismatch, exit_failed, direct
	Level  string            `json:"level"` // danger, warn or info
	Params map[string]string `json:"params"`
}

// ExitSummary is where the profile last went out to the internet.
type ExitSummary struct {
	IP      string `json:"ip"`
	Country string `json:"country"`
	At      string `json:"at"`
}

const maxErrorParam = 80

// exitFlags compares the newest measured session with the one before it and
// with the profile's timezone. Sessions on different proxies are not compared:
// the user changed the proxy on purpose.
func exitFlags(p store.Profile, latest, previous *store.Session) []Flag {
	flags := []Flag{}
	if p.ProxyID == "" {
		return append(flags, Flag{ID: "direct", Level: "info", Params: map[string]string{}})
	}
	if latest == nil || latest.ID == 0 {
		return flags
	}
	if latest.ExitError != "" {
		msg := latest.ExitError
		if len(msg) > maxErrorParam {
			msg = msg[:maxErrorParam] + "…"
		}
		return append(flags, Flag{ID: "exit_failed", Level: "warn", Params: map[string]string{"error": msg}})
	}
	if previous != nil && previous.ID != 0 && previous.ExitIP != "" && previous.ProxyID == latest.ProxyID {
		switch {
		case previous.ExitCountry != latest.ExitCountry:
			flags = append(flags, Flag{ID: "country_changed", Level: "danger",
				Params: map[string]string{"from": previous.ExitCountry, "to": latest.ExitCountry}})
		case previous.ExitIP != latest.ExitIP:
			flags = append(flags, Flag{ID: "ip_changed", Level: "info", Params: map[string]string{}})
		}
	}
	if latest.ExitTZ != "" && netproxy.CanonicalZone(latest.ExitTZ) != netproxy.CanonicalZone(p.Timezone) {
		flags = append(flags, Flag{ID: "tz_mismatch", Level: "warn",
			Params: map[string]string{"profile": p.Timezone, "ip": latest.ExitTZ}})
	}
	return flags
}

// Sessions lists the profile's 20 newest launches.
func (s *Service) Sessions(profileID string) ([]store.Session, error) {
	return s.Store.Sessions(profileID, 20)
}

// MatchExit checks the profile's proxy again and moves the profile's timezone,
// position and languages to where it exits now (the tz_mismatch fix).
func (s *Service) MatchExit(ctx context.Context, profileID string) error {
	p, err := s.Store.GetProfile(profileID)
	if err != nil {
		return err
	}
	if p.ProxyID != "" {
		if _, err := s.CheckProxyByID(ctx, p.ProxyID); err != nil {
			return err
		}
	}
	if err := s.applyNetwork(ctx, &p); err != nil {
		return err
	}
	if err := s.Store.UpdateProfile(p); err != nil {
		return err
	}
	s.notify("profiles")
	return nil
}
