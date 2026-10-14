// SPDX-License-Identifier: AGPL-3.0-or-later

// Package apostate runs Apostate (github.com/heretic-tech/apostate), a patched
// Chromium driven by command-line flags, as an AnyProfile engine. AnyProfile
// only downloads and executes the notarized release; it never modifies or
// redistributes it.
package apostate

// The pinned release. Hashes were checked against the downloaded files and the
// release's .manifest.json on 2026-10-14
// (plans/reports/research-261014-0001-apostate-integration.md).
const (
	Version  = "0.5.1"
	Chromium = "155.0.8059.31"
	// Catalogue is the persona-table digest of --fingerprint-explain. A
	// release with another digest re-draws every seed's machine; see the
	// upgrade rule in plans/reports/research-261014-0001-apostate-integration.md.
	Catalogue = "f7520eaac2212793d0ed147a82bf52303e77a8b18b91eb9e900c8e56acf0d117"

	zipName      = "apostate-155.0.8059.31-macos-arm64.zip"
	zipURL       = "https://github.com/heretic-tech/apostate/releases/download/v0.5.1/" + zipName
	zipSHA256    = "58176ba9b3e9cfd74029165bdf716c636e618af17a48bd905fe6d5f1552d901a"
	binarySHA256 = "2dfa8b148e6d06598981c4a1e1bc0d7c2c7860d74401ad0d934d21a10c7916d7"
	frameworkSHA = "f2a43605e500acb24cb7384f2b85f276c5f055713f565ff11ee8fdf1f6e41dba"
	teamID       = "PX3P7V373K"

	rootDir      = "apostate-155.0.8059.31-macos-arm64"
	appRel       = rootDir + "/Chromium.app"
	binRel       = appRel + "/Contents/MacOS/Chromium"
	frameworkRel = appRel + "/Contents/Frameworks/Chromium Framework.framework/Versions/155.0.8059.31/Chromium Framework"
)
