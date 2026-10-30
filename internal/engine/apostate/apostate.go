// SPDX-License-Identifier: AGPL-3.0-or-later

// Package apostate runs the official upstream Apostate release as a separate,
// optional engine. AnyBrowser remains the application's preferred engine.
package apostate

import (
	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/engine/anybrowser"
	"github.com/D0bby999/anyprofile/internal/paths"
)

const (
	Version      = "0.5.1"
	Chromium     = "155.0.8059.31"
	Catalogue    = "f7520eaac2212793d0ed147a82bf52303e77a8b18b91eb9e900c8e56acf0d117"
	HostPlatform = anybrowser.HostPlatform
	HostPersona  = "apostate-" + HostPlatform
)

// New reuses the compatible Chromium driver, but keeps Apostate's catalog,
// persona and installed binaries independent from AnyBrowser.
func New(dirs paths.Dirs) *anybrowser.AnyBrowser {
	return anybrowser.NewWithRelease(dirs, Builtin()[0], HostPersona, true)
}

// Builtin retains the verified upstream v0.5.1 transport and executable pins
// from before the AnyBrowser rename; never substitute the unpublished fork.
func Builtin() []engine.Release {
	return []engine.Release{{Engine: "apostate", Version: Version, Chromium: Chromium, Platform: hostPlat,
		Date: "2026-09-30", URL: zipURL, SHA256: zipSHA256,
		Files: map[string]string{binRel: binarySHA256, frameworkRel: frameworkSHA},
		Bin:   binRel, App: appRel, TeamID: teamID, Catalogue: Catalogue}}
}
