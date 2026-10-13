// SPDX-License-Identifier: AGPL-3.0-or-later

package fury

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/D0bby999/anyprofile/internal/engine"
)

// stamp remembers the binary's size and mtime at its last full verification,
// so startup re-hashes ~300 MB only when the file changed.
type stamp struct {
	Size    int64  `json:"size"`
	ModNano int64  `json:"modNano"`
	SHA256  string `json:"sha256"`
}

func (f *Fury) dir() string        { return f.dirs.Engine("fury", Version) }
func (f *Fury) BinaryPath() string { return filepath.Join(f.dir(), binRel) }
func (f *Fury) stampPath() string  { return filepath.Join(f.dir(), ".verified.json") }

func (f *Fury) Pinned() engine.Version {
	return engine.Version{Engine: "fury", Version: Version, Chromium: Chromium}
}

func (f *Fury) Status(ctx context.Context) engine.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	st := engine.Status{Version: f.Pinned()}
	fi, err := os.Stat(f.BinaryPath())
	if err != nil {
		return st
	}
	var s stamp
	if b, err := os.ReadFile(f.stampPath()); err == nil && json.Unmarshal(b, &s) == nil &&
		s.Size == fi.Size() && s.ModNano == fi.ModTime().UnixNano() && s.SHA256 == f.binarySHA {
		st.Installed = true
		return st
	}
	if err := engine.VerifySHA256(f.BinaryPath(), f.binarySHA); err != nil {
		st.Problem = "core binary does not match the pinned release; reinstall"
		return st
	}
	if err := f.writeStamp(); err != nil {
		st.Problem = err.Error()
		return st
	}
	st.Installed = true
	return st
}

func (f *Fury) writeStamp() error {
	fi, err := os.Stat(f.BinaryPath())
	if err != nil {
		return err
	}
	b, _ := json.Marshal(stamp{Size: fi.Size(), ModNano: fi.ModTime().UnixNano(), SHA256: f.binarySHA})
	return os.WriteFile(f.stampPath(), b, 0o600)
}

// Install downloads the pinned core and verifies it in order: tarball hash,
// extraction, binary hash, then Apple code signature and team. Nothing is
// moved into place until every check passes.
func (f *Fury) Install(ctx context.Context, progress engine.Progress) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	parent := filepath.Dir(f.dir())
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	tarball := filepath.Join(parent, tarballName)
	staging := filepath.Join(parent, ".staging-"+Version)
	defer os.RemoveAll(staging)

	if err := engine.Download(ctx, f.tarballURL, tarball, progress); err != nil {
		return err
	}
	defer os.Remove(tarball)
	if err := engine.VerifySHA256(tarball, f.tarballSHA); err != nil {
		return err
	}
	os.RemoveAll(staging)
	if err := engine.ExtractTarXZ(tarball, staging); err != nil {
		return fmt.Errorf("extract core: %w", err)
	}
	if err := engine.VerifySHA256(filepath.Join(staging, binRel), f.binarySHA); err != nil {
		return err
	}
	if !f.skipCodesign {
		if err := engine.VerifyCodesign(filepath.Join(staging, appName), teamID); err != nil {
			return err
		}
	}
	if err := os.RemoveAll(f.dir()); err != nil {
		return err
	}
	if err := os.Rename(staging, f.dir()); err != nil {
		return err
	}
	return f.writeStamp()
}

// CleanStale removes what an interrupted Install left behind: staging
// folders and the downloaded archive. The verified version folder is kept.
func (f *Fury) CleanStale() error {
	f.mu.Lock() // never while Install runs
	defer f.mu.Unlock()
	parent := filepath.Dir(f.dir())
	stale, _ := filepath.Glob(filepath.Join(parent, ".staging-*"))
	stale = append(stale, filepath.Join(parent, tarballName))
	for _, p := range stale {
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}
	return nil
}
