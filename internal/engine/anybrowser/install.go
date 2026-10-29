// SPDX-License-Identifier: AGPL-3.0-or-later

package anybrowser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/D0bby999/anyprofile/internal/engine"
)

func (a *AnyBrowser) dir(version string) string { return a.dirs.Engine("anybrowser", version) }

// SetCatalog makes the engine use c, which holds the signed list as well as
// the pinned release.
func (a *AnyBrowser) SetCatalog(c *engine.Catalog) { a.cat = c }

func (a *AnyBrowser) Releases() []engine.Release { return a.cat.For("anybrowser") }

func (a *AnyBrowser) release(version string) (engine.Release, error) {
	r, ok := a.cat.Find("anybrowser", version)
	if !ok {
		return r, fmt.Errorf("%w: anybrowser %s", engine.ErrUnknownRelease, version)
	}
	return r, nil
}

// AppPath is the pinned release's app: Chromium.app on macOS, the folder
// holding chrome.exe on Windows. Test rigs link an extracted release there.
func (a *AnyBrowser) AppPath() string { return filepath.Join(a.dir(Version), appRel) }

// Complete adds what the pinned release lacks on this OS to AppPath (see
// completeApp); test rigs that link an extracted release call it.
func (a *AnyBrowser) Complete() error { return completeApp(a.AppPath(), Chromium) }

func (a *AnyBrowser) Status(ctx context.Context, version string) engine.Status {
	r, err := a.release(version)
	if err != nil {
		return engine.Status{Version: engine.Version{Engine: "anybrowser", Version: version}, Problem: err.Error()}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	// Installs made before completeApp existed get the missing file now.
	return engine.CheckInstalled(r, a.dir(version), func(dir string) error {
		return completeApp(filepath.Join(dir, r.App), r.Chromium)
	})
}

// Install downloads a release and verifies it (engine.InstallRelease), then
// on macOS the Apple code signature and team, before anything is moved into
// place.
func (a *AnyBrowser) Install(ctx context.Context, version string, progress engine.Progress) error {
	r, err := a.release(version)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return engine.InstallRelease(ctx, r, a.dir(version), progress, func(tree string) error {
		app := filepath.Join(tree, r.App)
		if r.TeamID != "" {
			if err := engine.VerifyCodesign(app, r.TeamID); err != nil {
				return err
			}
		}
		return completeApp(app, r.Chromium)
	})
}

// Remove deletes an installed version. The caller makes sure no profile
// uses it.
func (a *AnyBrowser) Remove(version string) error {
	if _, err := a.release(version); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return os.RemoveAll(a.dir(version))
}

// CleanStale removes what an interrupted Install left behind.
func (a *AnyBrowser) CleanStale() error {
	a.mu.Lock() // never while Install runs
	defer a.mu.Unlock()
	return engine.CleanStaging(filepath.Dir(a.dir(Version)))
}
