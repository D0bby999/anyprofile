// SPDX-License-Identifier: AGPL-3.0-or-later

package fury

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/D0bby999/anyprofile/internal/engine"
)

func (f *Fury) dir(version string) string { return f.dirs.Engine("fury", version) }

// SetCatalog makes the engine use c, which holds the signed list as well as
// the compiled-in releases.
func (f *Fury) SetCatalog(c *engine.Catalog) { f.cat = c }

func (f *Fury) Releases() []engine.Release { return f.cat.For("fury") }

func (f *Fury) release(version string) (engine.Release, error) {
	r, ok := f.cat.Find("fury", version)
	if !ok {
		return r, fmt.Errorf("%w: fury %s", engine.ErrUnknownRelease, version)
	}
	return r, nil
}

func (f *Fury) Status(ctx context.Context, version string) engine.Status {
	r, err := f.release(version)
	if err != nil {
		return engine.Status{Version: engine.Version{Engine: "fury", Version: version}, Problem: err.Error()}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return engine.CheckInstalled(r, f.dir(version), nil)
}

// Install downloads a core release and verifies it (engine.InstallRelease),
// then the Apple code signature and team, before anything is moved into place.
func (f *Fury) Install(ctx context.Context, version string, progress engine.Progress) error {
	r, err := f.release(version)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return engine.InstallRelease(ctx, r, f.dir(version), progress, func(tree string) error {
		if f.skipCodesign {
			return nil
		}
		return engine.VerifyCodesign(filepath.Join(tree, r.App), r.TeamID)
	})
}

// Remove deletes an installed version. The caller makes sure no profile
// uses it.
func (f *Fury) Remove(version string) error {
	if _, err := f.release(version); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return os.RemoveAll(f.dir(version))
}

// CleanStale removes what an interrupted Install left behind.
func (f *Fury) CleanStale() error {
	f.mu.Lock() // never while Install runs
	defer f.mu.Unlock()
	return engine.CleanStaging(filepath.Dir(f.dir(Version)))
}
