// SPDX-License-Identifier: AGPL-3.0-or-later

package anybrowser

// The macOS arm64 build of AnyBrowser, the project's own Chromium fork
// (pro/chrome). On a Mac only the macOS persona is offered: the Windows
// persona renders with the host's fonts, which detection sites notice.
//
// The first AnyBrowser build is not hosted yet: zipURL is a placeholder and
// the hashes are zero. The publish step (docs/releasing.md) pins the real
// URL and hashes; until then installing this release fails closed at
// download/verification instead of fetching the old upstream build.
const (
	Version  = "0.5.1.156"
	Chromium = "156.0.8078.74"

	releaseDate        = "2026-10-31"
	legacyGoogleKeyEnv = false
	platform           = "macos"
	// hostPlat is the release's platform in the engine catalog.
	hostPlat  = "darwin-arm64"
	personaID = "anybrowser-macos"
	// setsTZ: the libc clock follows TZ, so the browser's own time code agrees
	// with --fingerprint-timezone.
	setsTZ = true

	zipName      = "anybrowser-156.0.8078.74-macos-arm64.zip"
	zipURL       = "https://example.invalid/anybrowser/" + zipName
	zipSHA256    = "d338a7c0169e053310c3749039dd126ceb179fd9a4ae2a71dc04d13a61b72d5f"
	binarySHA256 = "e78f52c48aa986391f7deefcaef615dc3dae80d952389766a2b65ee00ac54a2e"
	frameworkSHA = "23e66c35a184edfe700afe9138f70ebec2c521abcee3e58247d01bd787f463ef"
	// teamID is empty until the Developer ID signing identity is pinned at
	// publish; this local build is ad-hoc signed. The URL stays a placeholder,
	// so installing from the catalog still fails closed; the hashes let a
	// local extracted build be verified.
	teamID = ""

	rootDir = "anybrowser-156.0.8078.74-macos-arm64"
	appRel  = rootDir + "/Chromium.app"
	binRel  = appRel + "/Contents/MacOS/Chromium"
	// frameworkRel holds the browser's code; the launcher binary is a stub.
	frameworkRel = appRel + "/Contents/Frameworks/Chromium Framework.framework/Versions/156.0.8078.74/Chromium Framework"
)
