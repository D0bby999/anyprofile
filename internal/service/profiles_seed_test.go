// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/store"
)

// seedEngine is fakeEngine plus the optional composer, so the wiring that lets
// an engine draw its own seed is exercised without a browser.
type seedEngine struct{ engine.Engine }

func (seedEngine) ID() string { return "fury" }
func (seedEngine) Releases() []engine.Release {
	return []engine.Release{{Engine: "fury", Version: "9.9.9", Catalogue: "c"}}
}
func (seedEngine) Status(_ context.Context, v string) engine.Status {
	return engine.Status{Version: engine.Version{Engine: "fury", Version: v}, Installed: true}
}
func (seedEngine) UsesCookieKey() bool { return true }
func (seedEngine) NewIdentity(context.Context, uint64) (engine.Identity, error) {
	return engine.Identity{PersonaID: "unused", Platform: "macos"}, nil
}
func (seedEngine) ComposeIdentity(context.Context, string, uint64) (engine.Identity, error) {
	return engine.Identity{PersonaID: "macos-m2", Platform: "macos",
		Meta: store.EngineMeta{HWC: 8, MemGiB: 16}, SeedHex: "0123456789abcdef"}, nil
}

func TestCreateProfileUsesTheEnginesOwnSeed(t *testing.T) {
	s, _ := newService(t)
	s.Engines = map[string]engine.Engine{"fury": seedEngine{}}
	p, err := s.CreateProfile(context.Background(), NewProfile{Name: "Composed"})
	if err != nil {
		t.Fatal(err)
	}
	if p.SeedHex != "0123456789abcdef" {
		t.Fatalf("seed = %q, want the composed seed", p.SeedHex)
	}
	if p.EngineMeta.HWC != 8 || p.EngineMeta.MemGiB != 16 {
		t.Fatalf("engine meta = %+v, want the composed machine", p.EngineMeta)
	}
}

func TestCreateProfileDrawsASeedWithoutAComposer(t *testing.T) {
	s, _ := newService(t)
	p, err := s.CreateProfile(context.Background(), NewProfile{Name: "Drawn"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.SeedHex) != 16 {
		t.Fatalf("seed = %q, want 16 hex digits", p.SeedHex)
	}
}

// noSeedEngine composes an identity but leaves the seed to the caller, which is
// what AnyBrowser does when it cannot run the engine's explain mode.
type noSeedEngine struct{ seedEngine }

func (noSeedEngine) ComposeIdentity(context.Context, string, uint64) (engine.Identity, error) {
	return engine.Identity{PersonaID: "apostate-macos", Platform: "macos",
		Meta: store.EngineMeta{HWC: 8, MemGiB: 16}}, nil
}

func TestCreateProfileDrawsASeedWhenTheComposerReturnsNone(t *testing.T) {
	s, _ := newService(t)
	s.Engines = map[string]engine.Engine{"fury": noSeedEngine{}}
	p, err := s.CreateProfile(context.Background(), NewProfile{Name: "Fallback"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.SeedHex) != 16 {
		t.Fatalf("seed = %q, want 16 hex digits", p.SeedHex)
	}
	if p.EngineMeta.HWC != 8 || p.EngineMeta.MemGiB != 16 {
		t.Fatalf("engine meta = %+v, want the composed host machine", p.EngineMeta)
	}
}
