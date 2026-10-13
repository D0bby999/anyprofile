// SPDX-License-Identifier: AGPL-3.0-or-later

// Package engine defines what AnyProfile needs from a browser engine, and the
// download/verify helpers engines share. Each profile is bound to one engine
// at creation; fury is the only implementation in phase 1.
package engine

import (
	"context"

	"github.com/D0bby999/anyprofile/internal/store"
)

// Version identifies an installed engine build.
type Version struct {
	Engine   string `json:"engine"`
	Version  string `json:"version"`
	Chromium string `json:"chromium"`
}

// Status is what the UI shows about an engine.
type Status struct {
	Version
	Installed bool   `json:"installed"`
	Problem   string `json:"problem"` // set when files exist but fail verification
}

// Progress reports download progress; total is -1 when unknown.
type Progress func(done, total int64)

// Runtime is per-launch state the launcher prepares before BuildLaunch.
type Runtime struct {
	UserDataDir string
	RelayPort   int    // 0: no proxy, connect directly
	CDP         bool   // open a loopback DevTools port (diagnostics only)
	CDPPipe     bool   // DevTools over inherited pipes (fds 3 and 4); data pipes then start at fd 5
	CookieKey   []byte // 32 bytes, delivered on a pipe
	StartURLs   []string
}

// LaunchSpec is everything needed to exec the browser. Pipes[i] is written to
// a pipe the child inherits as fd 3+i, or 5+i when CDPPipe is set.
type LaunchSpec struct {
	Binary  string
	Args    []string
	Env     []string // appended to the parent's environment
	Pipes   [][]byte
	CDPPipe bool
}

// Engine is one browser engine implementation.
type Engine interface {
	ID() string
	Pinned() Version
	Status(ctx context.Context) Status
	Install(ctx context.Context, progress Progress) error
	BinaryPath() string
	// NewIdentity picks a persona for a new profile. roll is uniform random.
	NewIdentity(ctx context.Context, roll uint64) (personaID, platform string, err error)
	// PrepareProfile stages per-profile files (for example the Widevine CDM).
	PrepareProfile(p store.Profile, rt Runtime) error
	BuildLaunch(ctx context.Context, p store.Profile, rt Runtime) (LaunchSpec, error)
}
