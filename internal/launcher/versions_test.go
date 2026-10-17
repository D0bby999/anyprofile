// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

// versionsEngine has releases of two catalogues; installed says which are on disk.
type versionsEngine struct {
	engine.Engine
	installed map[string]bool
}

func (versionsEngine) ID() string { return "stub" }
func (versionsEngine) Releases() []engine.Release {
	return []engine.Release{
		{Engine: "stub", Version: "3.0", Chromium: "156.0.1.1", Catalogue: "a"},
		{Engine: "stub", Version: "2.0", Chromium: "155.0.1.1", Catalogue: "b"},
		{Engine: "stub", Version: "1.5", Chromium: "155.0.1.1", Catalogue: "a"},
		{Engine: "stub", Version: "1.0", Chromium: "155.0.1.1", Catalogue: "a"},
	}
}
func (e versionsEngine) Status(_ context.Context, v string) engine.Status {
	return engine.Status{Version: engine.Version{Engine: "stub", Version: v}, Installed: e.installed[v]}
}

func TestPickVersionMovesWithinACatalogue(t *testing.T) {
	dirs := paths.Dirs{Root: t.TempDir()}
	dirs.Ensure()
	st, err := store.Open(dirs.DB(), dirs.Backups())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for id, v := range map[string]string{"pa": "1.0", "pb": "2.0"} {
		if err := st.CreateProfile(&store.Profile{ID: id, Name: id, Engine: "stub", EngineVersion: v, PersonaID: "x",
			SeedHex: "00112233aabbccdd", Platform: "macos", Timezone: "UTC", Languages: "en-US", UILocale: "en-US"}); err != nil {
			t.Fatal(err)
		}
	}
	st.SaveFingerprint("pa", `{"ua":"old"}`, `{"ua":"old"}`, "baseline", nil)
	eng := versionsEngine{installed: map[string]bool{"1.0": true, "1.5": true, "2.0": true}}
	l := New(st, secrets.NewMemory(), dirs, []engine.Engine{eng}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	pick := func(id string) store.Profile {
		t.Helper()
		p, _ := st.GetProfile(id)
		if err := l.pickVersion(ctx, eng, &p); err != nil {
			t.Fatal(err)
		}
		saved, _ := st.GetProfile(id)
		if saved.EngineVersion != p.EngineVersion {
			t.Fatalf("%s: picked %s, saved %s", id, p.EngineVersion, saved.EngineVersion)
		}
		return saved
	}
	// 3.0 is not installed and 2.0 is another catalogue: 1.0 moves to 1.5,
	// the same Chromium, so its fingerprint baseline stays.
	if p := pick("pa"); p.EngineVersion != "1.5" {
		t.Fatalf("pa on %s", p.EngineVersion)
	}
	if b, _ := st.FingerprintBaseline("pa"); b == "" {
		t.Fatal("baseline dropped without a Chromium change")
	}
	if p := pick("pb"); p.EngineVersion != "2.0" {
		t.Fatalf("pb moved across catalogues to %s", p.EngineVersion)
	}
	// 3.0 installed: a new Chromium, so the baseline starts again.
	eng.installed["3.0"] = true
	if p := pick("pa"); p.EngineVersion != "3.0" {
		t.Fatalf("pa on %s", p.EngineVersion)
	}
	if b, _ := st.FingerprintBaseline("pa"); b != "" {
		t.Fatal("baseline kept across a Chromium change")
	}
	// Its own version gone, nothing to move to: refused with engine_missing.
	delete(eng.installed, "2.0")
	p, _ := st.GetProfile("pb")
	if err := l.pickVersion(ctx, eng, &p); err == nil {
		t.Fatal("opened on a version that is not installed")
	}
}
