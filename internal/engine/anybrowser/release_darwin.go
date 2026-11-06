// SPDX-License-Identifier: AGPL-3.0-or-later

package anybrowser

import (
	"os"
	"strings"
)

// The macOS arm64 build of AnyBrowser, the project's own Chromium fork
// (pro/chrome). On a Mac only the macOS persona is offered: the Windows
// persona renders with the host's fonts, which detection sites notice.
//
// The engine archive is hosted on its own pre-release and signed with the same
// Developer ID as the app. The publish step (docs/releasing.md) pins the URL and
// hashes; a build that is not hosted yet keeps the placeholder URL and ad-hoc
// hashes, so installs fail closed instead of fetching some other build.
const (
	Version  = "0.5.1.157"
	Chromium = "156.0.8078.74"

	releaseDate        = "2026-11-07"
	legacyGoogleKeyEnv = false
	platform           = "macos"
	// hostPlat is the release's platform in the engine catalog.
	hostPlat  = "darwin-arm64"
	personaID = "anybrowser-macos"
	// setsTZ: the libc clock follows TZ, so the browser's own time code agrees
	// with --fingerprint-timezone.
	setsTZ = true
	// supportsGeolocation: patch 0164 answers navigator.geolocation with
	// --fingerprint-geolocation instead of CoreLocation.
	supportsGeolocation = true

	zipName = "anybrowser-156.0.8078.74-macos-arm64.zip"
	// engineTag names the pre-release the archive is attached to.
	engineTag     = "engine-anybrowser-0.5.1.157"
	defaultZipURL = "https://github.com/D0bby999/anyprofile/releases/download/" + engineTag + "/" + zipName
	zipSHA256     = "d022da32a82a078eedb1f7ab6730d07e0dfd1eb567025edc69885fa4023a34c5"
	binarySHA256  = "da649b02d3daed2cd2b5785e91ca9aaf60c4e727036aa756415adeda80736849"
	frameworkSHA  = "28e9691caf659331de5a617b67519ce7f1f418e7153dff3b81cdd9ca572e2933"
	// teamID is the Developer ID team the bundle's signature must name.
	teamID = "25C8SNXS6Q"

	rootDir = "anybrowser-156.0.8078.74-macos-arm64"
	appRel  = rootDir + "/Chromium.app"
	binRel  = appRel + "/Contents/MacOS/Chromium"
	// frameworkRel holds the browser's code; the launcher binary is a stub.
	frameworkRel = appRel + "/Contents/Frameworks/Chromium Framework.framework/Versions/156.0.8078.74/Chromium Framework"
)

// zipURL is defaultZipURL unless a development shell points the app at a
// local mirror of the exact pinned zip (scripts/dev-anybrowser-server.sh).
// The SHA256 below still has to match; a server cannot substitute another
// build.
var zipURL = defaultZipURL

func init() {
	if v := strings.TrimSpace(os.Getenv("ANYPROFILE_ANYBROWSER_URL")); v != "" {
		zipURL = v
	}
}
