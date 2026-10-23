// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/netproxy"
	"github.com/D0bby999/anyprofile/internal/secrets"
)

// exitNetwork is what a proxy's exit implies for whatever goes through it:
// the timezone, position and languages a person there would have.
type exitNetwork struct {
	Timezone  string
	Country   string
	Lat, Lon  *float64
	UILocale  string
	Languages []string
}

// proxyNetwork measures proxyID's exit when it never was, and refuses a
// rotating proxy or one without a known timezone. Without a proxy it is this
// computer's own zone. Profiles and mobile devices share it.
func (s *Service) proxyNetwork(ctx context.Context, proxyID string) (exitNetwork, error) {
	var n exitNetwork
	if proxyID == "" {
		n.Timezone = localZone()
	} else {
		px, err := s.Store.GetProxy(proxyID)
		if err != nil {
			return n, fmt.Errorf("proxy: %w", err)
		}
		if px.CheckedAt == "" {
			if px, err = s.CheckProxyByID(ctx, px.ID); err != nil {
				return n, err
			}
		}
		if px.Rotating {
			return n, apperr.Newf("proxy_rotating", "proxy %s changes its IP between connections; use a sticky port", px.Label)
		}
		if px.Timezone == "" {
			return n, apperr.Newf("proxy_no_timezone", "could not determine the timezone of proxy %s", px.Label)
		}
		n.Timezone, n.Lat, n.Lon, n.Country = px.Timezone, px.Lat, px.Lon, px.Country
	}
	ui, langs, err := s.Locale(ctx, n.Country)
	if err != nil {
		return n, err
	}
	n.UILocale, n.Languages = ui, langs
	return n, nil
}

// proxyUpstream is the proxy with its password, for this process only.
func (s *Service) proxyUpstream(proxyID string) (netproxy.Upstream, error) {
	px, err := s.Store.GetProxy(proxyID)
	if err != nil {
		return netproxy.Upstream{}, err
	}
	up := netproxy.Upstream{Scheme: px.Scheme, Host: px.Host, Port: px.Port, Username: px.Username}
	switch pw, err := s.Secrets.Get(secrets.ServiceProxy, proxyID); {
	case err == nil:
		up.Password = string(pw)
	case !errors.Is(err, secrets.ErrNotFound):
		return up, fmt.Errorf("read proxy password: %w", err)
	}
	return up, nil
}
