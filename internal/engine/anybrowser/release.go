// SPDX-License-Identifier: AGPL-3.0-or-later

// Package anybrowser runs the platform-pinned Chromium browser as an AnyProfile
// engine. AnyProfile downloads and executes the project's fork on
// macOS, and the existing upstream Apostate release on Windows until a Windows
// fork has actually passed release acceptance (GPL-3.0, see NOTICE).
package anybrowser

import "github.com/D0bby999/anyprofile/internal/engine"

// Version, Chromium and transport pins are platform-specific. The unpublished
// macOS fork must not replace the working Windows upstream pin with placeholders.
const (
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
		Date: releaseDate, URL: zipURL, SHA256: zipSHA256,
		Files: map[string]string{binRel: binarySHA256, frameworkRel: frameworkSHA},
		Bin:   binRel, App: appRel, TeamID: teamID, Catalogue: Catalogue}}
}
