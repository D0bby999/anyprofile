// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"

	"github.com/D0bby999/anyprofile/internal/store"
)

func proxyProfile(t *testing.T, s *Service, proxyID string, lat, lon float64) store.Profile {
	t.Helper()
	p := store.Profile{ID: "p-" + proxyID, Name: "p", Engine: "fury", EngineVersion: "1", PersonaID: "macos", SeedHex: "0000000000000001",
		ProxyID: proxyID, Timezone: "America/Los_Angeles", Lat: &lat, Lon: &lon}
	if err := s.Store.CreateProfile(&p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRefreshLaunchIdentityFollowsTheExit(t *testing.T) {
	s, _ := newService(t)
	px := store.Proxy{ID: "px1", Label: "sticky", Scheme: "http", Host: "gw", Port: 10002, Username: "u"}
	if err := s.Store.CreateProxy(&px); err != nil {
		t.Fatal(err)
	}
	p := proxyProfile(t, s, px.ID, 37.39, -121.96)
	s.RefreshLaunchIdentity(context.Background(), p.ID)
	got, err := s.Store.GetProfile(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Lat == nil || got.Lon == nil || *got.Lat != 10.8 || *got.Lon != 106.6 {
		t.Fatalf("position = %v,%v, want the exit's", got.Lat, got.Lon)
	}
	if got.Timezone != "Asia/Ho_Chi_Minh" {
		t.Fatalf("timezone = %q, want the exit's", got.Timezone)
	}
}

func TestRefreshLaunchIdentityDropsAPositionItCannotVerify(t *testing.T) {
	s, _ := newService(t)
	rotating := store.Proxy{ID: "px2", Label: "rotating", Scheme: "http", Host: "gw", Port: 823, Username: "u"}
	if err := s.Store.CreateProxy(&rotating); err != nil {
		t.Fatal(err)
	}
	p := proxyProfile(t, s, rotating.ID, 37.39, -121.96)
	s.RefreshLaunchIdentity(context.Background(), p.ID)
	got, _ := s.Store.GetProfile(p.ID)
	if got.Lat != nil || got.Lon != nil {
		t.Fatalf("position = %v,%v, want none for a rotating exit", got.Lat, got.Lon)
	}
}
