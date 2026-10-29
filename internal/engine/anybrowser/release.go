// SPDX-License-Identifier: AGPL-3.0-or-later

// Package anybrowser runs AnyBrowser, the patched Chromium built from the
// project's own fork (pro/chrome), as an AnyProfile engine. AnyProfile
// downloads the project's own release build and executes it; the fork's
// fingerprint code derives from the Apostate project (GPL-3.0, see NOTICE).
package anybrowser

import "github.com/D0bby999/anyprofile/internal/engine"

// The pinned release: the fork's Chromium 156 build. Its archive is not
// hosted yet, so the per-OS URL and hashes in release_<os>.go are
// placeholders that the publish step replaces (docs/releasing.md). Each
// OS's build is in release_<os>.go; both carry the same persona tables.
const (
	Version  = "0.5.1.156"
	Chromium = "156.0.8078.48"
	// Catalogue is the persona-table compatibility digest. A release with
	// another digest re-draws every seed's machine, so the fork keeps the
	// digest of the tables it ships (pro/chrome/build/CATALOGUE_ID); see
	// the upgrade rule in plans/reports/research-261014-0001-apostate-integration.md.
	Catalogue = "f7520eaac2212793d0ed147a82bf52303e77a8b18b91eb9e900c8e56acf0d117"
)

// HostPlatform and HostPersona are the persona this build offers: the OS it
// runs on (release_<os>.go).
const (
	HostPlatform = platform
	HostPersona  = personaID
)

// Builtin is the pinned release for this OS, as catalog data.
func Builtin() []engine.Release {
	return []engine.Release{{Engine: "anybrowser", Version: Version, Chromium: Chromium, Platform: hostPlat,
		Date: "2026-10-24", URL: zipURL, SHA256: zipSHA256,
		Files: map[string]string{binRel: binarySHA256, frameworkRel: frameworkSHA},
		Bin:   binRel, App: appRel, TeamID: teamID, Catalogue: Catalogue}}
}
