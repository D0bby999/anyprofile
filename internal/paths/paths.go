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

// EnvDataDir overrides the data directory with an absolute path.
// Development and acceptance runs only; lets a fresh install be tested
// without touching the user's data.
const EnvDataDir = "ANYPROFILE_DATA_DIR"

// Default is ~/Library/Application Support/AnyProfile on macOS, or the
// absolute path in ANYPROFILE_DATA_DIR. A relative override is ignored.
func Default() (Dirs, error) {
	if v := os.Getenv(EnvDataDir); v != "" && filepath.IsAbs(v) {
		return Dirs{Root: filepath.Clean(v)}, nil
	}
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
func (d Dirs) Snapshots() string            { return filepath.Join(d.Root, "snapshots") }
func (d Dirs) ProfileSnapshots(id string) string {
	return filepath.Join(d.Snapshots(), id)
}

// Extensions holds the unpacked extensions of the library, one folder each.
func (d Dirs) Extensions() string         { return filepath.Join(d.Root, "extensions") }
func (d Dirs) Extension(id string) string { return filepath.Join(d.Extensions(), id) }

// Uploads holds the only files an AI agent may put into a page's file input.
func (d Dirs) Uploads() string { return filepath.Join(d.Root, "uploads") }

// Recipes holds the agent recipes (*.md) the MCP endpoint offers.
func (d Dirs) Recipes() string { return filepath.Join(d.Root, "recipes") }

// Ensure creates every directory with owner-only permissions, tightening any that exist.
func (d Dirs) Ensure() error {
	for _, p := range []string{d.Root, d.Engines(), d.Profiles(), d.Logs(), d.Backups(), d.Snapshots(), d.Extensions(), d.Uploads(), d.Recipes()} {
		if err := os.MkdirAll(p, 0o700); err != nil {
			return fmt.Errorf("create %s: %w", p, err)
		}
		if err := os.Chmod(p, 0o700); err != nil {
			return fmt.Errorf("chmod %s: %w", p, err)
		}
	}
	return nil
}
