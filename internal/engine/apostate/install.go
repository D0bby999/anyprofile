// SPDX-License-Identifier: AGPL-3.0-or-later

package apostate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/D0bby999/anyprofile/internal/engine"
)

// stamp remembers the framework's size and mtime at its last full
// verification, so startup re-hashes its ~235 MB only when the file changed.
// The framework holds the browser's code; the launcher binary is a stub.
type stamp struct {
	Size    int64  `json:"size"`
	ModNano int64  `json:"modNano"`
	SHA256  string `json:"sha256"`
}

func (a *Apostate) dir() string        { return a.dirs.Engine("apostate", Version) }
func (a *Apostate) BinaryPath() string { return filepath.Join(a.dir(), binRel) }

// AppPath is the installed app: Chromium.app on macOS, the folder holding
// chrome.exe on Windows.
func (a *Apostate) AppPath() string       { return filepath.Join(a.dir(), appRel) }
func (a *Apostate) frameworkPath() string { return filepath.Join(a.dir(), frameworkRel) }
func (a *Apostate) stampPath() string     { return filepath.Join(a.dir(), ".verified.json") }

func (a *Apostate) Pinned() engine.Version {
	return engine.Version{Engine: "apostate", Version: Version, Chromium: Chromium}
}

func (a *Apostate) Status(ctx context.Context) engine.Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := engine.Status{Version: a.Pinned()}
	if _, err := os.Stat(a.BinaryPath()); err != nil {
		return st
	}
	fi, err := os.Stat(a.frameworkPath())
	if err != nil {
		return st
	}
	var s stamp
	if b, err := os.ReadFile(a.stampPath()); err == nil && json.Unmarshal(b, &s) == nil &&
		s.Size == fi.Size() && s.ModNano == fi.ModTime().UnixNano() && s.SHA256 == frameworkSHA {
		st.Installed = true
		return st
	}
	if engine.VerifySHA256(a.BinaryPath(), binarySHA256) != nil || engine.VerifySHA256(a.frameworkPath(), frameworkSHA) != nil {
		st.Problem = "engine files do not match the pinned release; reinstall"
		return st
	}
	if err := a.writeStamp(); err != nil {
		st.Problem = err.Error()
		return st
	}
	st.Installed = true
	return st
}

func (a *Apostate) writeStamp() error {
	fi, err := os.Stat(a.frameworkPath())
	if err != nil {
		return err
	}
	b, _ := json.Marshal(stamp{Size: fi.Size(), ModNano: fi.ModTime().UnixNano(), SHA256: frameworkSHA})
	return os.WriteFile(a.stampPath(), b, 0o600)
}

// Install downloads the pinned release and verifies it in order: zip hash,
// extraction, binary hash, framework hash, then (macOS) Apple code signature
// and team.
// Nothing is moved into place until every check passes.
func (a *Apostate) Install(ctx context.Context, progress engine.Progress) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	parent := filepath.Dir(a.dir())
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	staging := filepath.Join(parent, ".staging-"+Version)
	os.RemoveAll(staging)
	defer os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return err
	}
	zipPath := filepath.Join(staging, zipName)
	if err := engine.Download(ctx, zipURL, zipPath, progress); err != nil {
		return err
	}
	return a.installFrom(zipPath, staging)
}

// installFrom verifies and installs a downloaded zip that sits in staging.
func (a *Apostate) installFrom(zipPath, staging string) error {
	if err := engine.VerifySHA256(zipPath, zipSHA256); err != nil {
		return err
	}
	tree := filepath.Join(staging, "tree")
	if err := engine.ExtractZip(zipPath, tree); err != nil {
		return fmt.Errorf("extract engine: %w", err)
	}
	if err := os.Remove(zipPath); err != nil {
		return err
	}
	if err := engine.VerifySHA256(filepath.Join(tree, binRel), binarySHA256); err != nil {
		return err
	}
	if err := engine.VerifySHA256(filepath.Join(tree, frameworkRel), frameworkSHA); err != nil {
		return err
	}
	if err := verifyApp(filepath.Join(tree, appRel)); err != nil {
		return err
	}
	if err := os.RemoveAll(a.dir()); err != nil {
		return err
	}
	if err := os.Rename(tree, a.dir()); err != nil {
		return err
	}
	return a.writeStamp()
}

// CleanStale removes what an interrupted Install left behind: staging
// folders and stray archives. The verified version folder is kept.
func (a *Apostate) CleanStale() error {
	a.mu.Lock() // never while Install runs
	defer a.mu.Unlock()
	parent := filepath.Dir(a.dir())
	stale, _ := filepath.Glob(filepath.Join(parent, ".staging-*"))
	zips, _ := filepath.Glob(filepath.Join(parent, "*.zip"))
	for _, p := range append(stale, zips...) {
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}
	return nil
}
