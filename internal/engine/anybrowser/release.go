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

// Builtin is the pinned release for this OS, as catalog data, newest first.
//
// The release this build replaces rides along. pickVersion only moves a profile
// onto a newer build it can find in this list, so dropping the old entry would
// leave every profile created on it on the old engine — which is exactly where a
// fix like the display-scale one would never arrive.
func Builtin() []engine.Release {
	current := engine.Release{Engine: "anybrowser", Version: Version, Chromium: Chromium, Platform: hostPlat,
		Date: releaseDate, URL: zipURL, SHA256: zipSHA256,
		Files: map[string]string{binRel: binarySHA256, frameworkRel: frameworkSHA},
		Bin:   binRel, App: appRel, TeamID: teamID, Catalogue: Catalogue}
	if previousVersion == "" {
		return []engine.Release{current}
	}
	previous := current
	previous.Version = previousVersion
	previous.Date = previousDate
	previous.URL = "https://github.com/D0bby999/anyprofile/releases/download/engine-anybrowser-" +
		previousVersion + "/" + zipName
	previous.SHA256 = previousZipSHA256
	previous.Files = map[string]string{binRel: previousBinarySHA256, frameworkRel: previousFrameworkSHA}
	return []engine.Release{current, previous}
}
