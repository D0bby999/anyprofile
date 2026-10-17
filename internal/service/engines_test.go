// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"

	"github.com/D0bby999/anyprofile/internal/engine"
)

// shelfEngine has two releases of one catalogue and one of another.
type shelfEngine struct {
	fakeEngine
	installed map[string]bool
	removed   []string
}

func (*shelfEngine) Releases() []engine.Release {
	return []engine.Release{
		{Engine: "fury", Version: "0.4.5", Chromium: "155.0.8059.39", Catalogue: "a"},
		{Engine: "fury", Version: "0.4.0", Chromium: "155.0.8059.39", Catalogue: "b"},
		{Engine: "fury", Version: "0.3.5", Chromium: "155.0.8059.39", Catalogue: "a"},
	}
}
func (e *shelfEngine) Status(_ context.Context, v string) engine.Status {
	return engine.Status{Version: engine.Version{Engine: "fury", Version: v}, Installed: e.installed[v]}
}
func (e *shelfEngine) Remove(v string) error {
	e.removed = append(e.removed, v)
	delete(e.installed, v)
	return nil
}

func TestRemoveEngineVersion(t *testing.T) {
	s, _ := newService(t)
	eng := &shelfEngine{installed: map[string]bool{"0.3.5": true, "0.4.0": true}}
	s.Engines = map[string]engine.Engine{"fury": eng}
	ctx := context.Background()
	// New profiles get the newest installed version, not a newer one still to download.
	p, err := s.CreateProfile(ctx, NewProfile{Name: "Old"})
	if err != nil {
		t.Fatal(err)
	}
	if p.EngineVersion != "0.4.0" {
		t.Fatalf("new profile on %s", p.EngineVersion)
	}
	s.Store.SetEngineVersion(p.ID, "0.3.5")
	s.Store.SoftDelete(p.ID)

	// 0.3.5 is used (from the trash too) and nothing of its catalogue is installed.
	if err := s.RemoveEngineVersion(ctx, "fury", "0.3.5"); err == nil {
		t.Fatal("removed a version its profiles need")
	}
	rels, _ := s.EngineReleases(ctx, "fury")
	if len(rels) != 3 || rels[2].Profiles != 1 || !rels[2].Installed {
		t.Fatalf("releases = %+v", rels)
	}
	// With 0.4.5 installed, the profile moves there and 0.3.5 goes.
	eng.installed["0.4.5"] = true
	if err := s.RemoveEngineVersion(ctx, "fury", "0.3.5"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Store.GetProfile(p.ID)
	if got.EngineVersion != "0.4.5" || len(eng.removed) != 1 {
		t.Fatalf("profile on %s, removed %v", got.EngineVersion, eng.removed)
	}
	if err := s.RemoveEngineVersion(ctx, "fury", "9.9.9"); err == nil {
		t.Fatal("removed an unknown version")
	}
}

func TestPruneEngines(t *testing.T) {
	s, _ := newService(t)
	eng := &shelfEngine{installed: map[string]bool{"0.3.5": true}}
	s.Engines = map[string]engine.Engine{"fury": eng}
	ctx := context.Background()
	// The only version installed stays, used or not.
	s.PruneEngines(ctx)
	if len(eng.removed) != 0 {
		t.Fatalf("removed %v", eng.removed)
	}
	// A profile in the trash still needs 0.4.0; 0.3.5 is unused once 0.4.5 is there.
	eng.installed["0.4.0"], eng.installed["0.4.5"] = true, true
	p, err := s.CreateProfile(ctx, NewProfile{Name: "Trashed"})
	if err != nil {
		t.Fatal(err)
	}
	s.Store.SetEngineVersion(p.ID, "0.4.0")
	s.Store.SoftDelete(p.ID)
	s.PruneEngines(ctx)
	if len(eng.removed) != 1 || eng.removed[0] != "0.3.5" || !eng.installed["0.4.0"] || !eng.installed["0.4.5"] {
		t.Fatalf("removed %v, left %v", eng.removed, eng.installed)
	}
}
