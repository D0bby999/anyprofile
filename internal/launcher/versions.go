// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"context"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/store"
)

// pickVersion settles the engine version prof opens on. A newer installed
// release of the same catalogue draws the same machine from the profile's
// seed, so the profile moves to it and keeps it, as Chrome updates itself;
// with a new Chromium its fingerprint baseline starts again, since the user
// agent changes. A release of another catalogue would be another machine:
// that move is the user's, never made here.
func (l *Launcher) pickVersion(ctx context.Context, eng engine.Engine, prof *store.Profile) error {
	rels := eng.Releases()
	cur := -1
	for i, r := range rels {
		if r.Version == prof.EngineVersion {
			cur = i
		}
	}
	for i := 0; cur > 0 && i < cur; i++ { // newest first: only the newer ones
		r := rels[i]
		if r.Catalogue != rels[cur].Catalogue || !eng.Status(ctx, r.Version).Installed {
			continue
		}
		l.log.Info("engine version moves", "profile", prof.ID, "engine", prof.Engine, "from", prof.EngineVersion, "to", r.Version)
		if err := l.st.SetEngineVersion(prof.ID, r.Version); err != nil {
			return err
		}
		prof.EngineVersion = r.Version
		if r.Chromium != rels[cur].Chromium {
			if err := l.st.ResetFingerprint(prof.ID); err != nil {
				l.log.Warn("reset fingerprint after engine move", "profile", prof.ID, "err", err)
			}
		}
		break
	}
	if st := eng.Status(ctx, prof.EngineVersion); !st.Installed {
		if st.Problem != "" {
			return apperr.Newf("engine_missing", "engine %s %s: %s", prof.Engine, prof.EngineVersion, st.Problem)
		}
		return apperr.Newf("engine_missing", "engine %s %s is not installed", prof.Engine, prof.EngineVersion)
	}
	return nil
}

// releaseChromium is the Chromium version of an engine release, "" if unknown.
func releaseChromium(eng engine.Engine, version string) string {
	for _, r := range eng.Releases() {
		if r.Version == version {
			return r.Chromium
		}
	}
	return ""
}
