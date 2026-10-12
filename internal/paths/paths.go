// SPDX-License-Identifier: AGPL-3.0-or-later

// Package paths owns the on-disk layout of AnyProfile's data directory.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// Dirs is the data layout rooted at one directory.
type Dirs struct {
	Root string
}

// Default is ~/Library/Application Support/AnyProfile on macOS.
func Default() (Dirs, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return Dirs{}, err
	}
	return Dirs{Root: filepath.Join(base, "AnyProfile")}, nil
}

func (d Dirs) DB() string                   { return filepath.Join(d.Root, "anyprofile.db") }
func (d Dirs) Engines() string              { return filepath.Join(d.Root, "engines") }
func (d Dirs) Engine(id, ver string) string { return filepath.Join(d.Engines(), id, ver) }
func (d Dirs) Profiles() string             { return filepath.Join(d.Root, "profiles") }
func (d Dirs) Profile(id string) string     { return filepath.Join(d.Profiles(), id) }
func (d Dirs) Logs() string                 { return filepath.Join(d.Root, "logs") }
func (d Dirs) Backups() string              { return filepath.Join(d.Root, "backups") }

// Ensure creates every directory with owner-only permissions, tightening any that exist.
func (d Dirs) Ensure() error {
	for _, p := range []string{d.Root, d.Engines(), d.Profiles(), d.Logs(), d.Backups()} {
		if err := os.MkdirAll(p, 0o700); err != nil {
			return fmt.Errorf("create %s: %w", p, err)
		}
		if err := os.Chmod(p, 0o700); err != nil {
			return fmt.Errorf("chmod %s: %w", p, err)
		}
	}
	return nil
}
