// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/store"
)

// EngineRelease is one version of an engine as Settings shows it.
type EngineRelease struct {
	engine.Status
	Date     string `json:"date"`
	Notes    string `json:"notes"`
	Profiles int    `json:"profiles"` // profiles on this version, the trash included
}

// EngineReleases lists an engine's versions for this computer, newest first.
func (s *Service) EngineReleases(ctx context.Context, id string) ([]EngineRelease, error) {
	eng, err := s.engineFor(id)
	if err != nil {
		return nil, err
	}
	counts, err := s.versionCounts(eng.ID())
	if err != nil {
		return nil, err
	}
	var out []EngineRelease
	for _, r := range eng.Releases() {
		out = append(out, EngineRelease{Status: eng.Status(ctx, r.Version), Date: r.Date, Notes: r.Notes,
			Profiles: counts[r.Version]})
	}
	return out, nil
}

// EngineState is the version an engine works with now: the newest installed
// one, or the newest when none is.
func (s *Service) EngineState(ctx context.Context, eng engine.Engine) engine.Status {
	rels := eng.Releases()
	for _, r := range rels {
		if st := eng.Status(ctx, r.Version); st.Installed {
			return st
		}
	}
	if len(rels) == 0 {
		return engine.Status{Version: engine.Version{Engine: eng.ID()}, Problem: "no release for this computer"}
	}
	return eng.Status(ctx, rels[0].Version)
}

// newProfileVersion is what a new profile gets: the newest installed release,
// so a newer one in the signed list never stops profiles from being made.
func (s *Service) newProfileVersion(ctx context.Context, eng engine.Engine) string {
	return s.EngineState(ctx, eng).Version.Version
}

func (s *Service) versionCounts(engineID string) (map[string]int, error) {
	counts := map[string]int{}
	for _, trashed := range []bool{false, true} {
		ps, err := s.Store.ListProfiles(store.ProfileFilter{Trashed: trashed})
		if err != nil {
			return nil, err
		}
		for _, p := range ps {
			if p.Engine == engineID {
				counts[p.EngineVersion]++
			}
		}
	}
	return counts, nil
}

// RemoveEngineVersion deletes an installed version. Its profiles move to the
// newest installed release of the same catalogue, which draws the same
// machine; without one, or while one of them is open, it is refused.
func (s *Service) RemoveEngineVersion(ctx context.Context, id, version string) error {
	eng, err := s.engineFor(id)
	if err != nil {
		return err
	}
	rels := eng.Releases()
	var gone *engine.Release
	for i := range rels {
		if rels[i].Version == version {
			gone = &rels[i]
		}
	}
	if gone == nil {
		return apperr.Newf("bad_engine_version", "%s has no version %s", id, version)
	}
	target := ""
	for _, r := range rels {
		if r.Version != version && r.Catalogue == gone.Catalogue && eng.Status(ctx, r.Version).Installed {
			target = r.Version
			break
		}
	}
	var users []store.Profile
	for _, trashed := range []bool{false, true} {
		ps, err := s.Store.ListProfiles(store.ProfileFilter{Trashed: trashed})
		if err != nil {
			return err
		}
		for _, p := range ps {
			if p.Engine == id && p.EngineVersion == version {
				users = append(users, p)
			}
		}
	}
	if len(users) > 0 && target == "" {
		return apperr.Newf("engine_version_in_use", "%d profiles use %s %s; install a newer version first", len(users), id, version)
	}
	for _, p := range users {
		if _, open := s.Launcher.Info(p.ID); open {
			return apperr.Newf("profiles_open", "close %s first: it runs on this version", p.Name)
		}
	}
	for _, p := range users {
		if err := s.Store.SetEngineVersion(p.ID, target); err != nil {
			return err
		}
		if r, _ := findRelease(rels, target); r.Chromium != gone.Chromium {
			_ = s.Store.ResetFingerprint(p.ID) // a new user agent: the next launch measures afresh
		}
	}
	if len(users) > 0 {
		s.Log.Info("engine version removed; profiles moved", "engine", id, "from", version, "to", target, "profiles", len(users))
		s.notify("profiles")
	}
	return eng.Remove(version)
}

func findRelease(rels []engine.Release, version string) (engine.Release, bool) {
	for _, r := range rels {
		if r.Version == version {
			return r, true
		}
	}
	return engine.Release{}, false
}
