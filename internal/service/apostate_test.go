// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"

	"github.com/D0bby999/anyprofile/internal/engine/anybrowser"
	"github.com/D0bby999/anyprofile/internal/engine/apostate"
)

func TestApostateOptionKeepsAnyBrowserDefault(t *testing.T) {
	s, _ := newService(t)
	s.Engines["anybrowser"] = anybrowser.New(s.Dirs)
	s.Engines["apostate"] = apostate.New(s.Dirs)
	s.Preferred = "anybrowser"
	ctx := context.Background()
	p, err := s.CreateProfile(ctx, NewProfile{Name: "Default", Sites: []string{"other"}})
	if err != nil || p.Engine != "anybrowser" || s.DefaultEngine() != "anybrowser" {
		t.Fatalf("default profile = %+v, error = %v", p, err)
	}
	a, err := s.CreateProfile(ctx, NewProfile{Name: "Upstream", Engine: "apostate", Sites: []string{"other"}})
	if err != nil || a.Engine != "apostate" || a.EngineVersion != apostate.Version || a.PersonaID != apostate.HostPersona {
		t.Fatalf("Apostate profile = %+v, error = %v", a, err)
	}
	bulk, err := s.CreateProfiles(ctx, BulkCreate{BaseName: "Upstream bulk", Engine: "apostate", Count: 2,
		Proxy: "none", Sites: []string{"other"}})
	if err != nil || len(bulk) != 2 {
		t.Fatalf("bulk = %+v, error = %v", bulk, err)
	}
	for _, row := range bulk {
		if row.Engine != "apostate" || row.PersonaID != apostate.HostPersona {
			t.Fatalf("bulk profile = %+v", row)
		}
	}
	stored, err := s.Store.GetProfile(p.ID)
	if err != nil || stored.Engine != "anybrowser" || s.DefaultEngine() != "anybrowser" {
		t.Fatalf("existing profile/default changed: %+v, error = %v", stored, err)
	}
	rels, err := s.EngineReleases(ctx, "apostate")
	if err != nil || len(rels) != 1 || rels[0].Profiles != 3 || rels[0].Engine != "apostate" {
		t.Fatalf("Apostate releases = %+v, error = %v", rels, err)
	}
}
