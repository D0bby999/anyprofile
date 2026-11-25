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
	Version  = "0.5.1.159"
	Chromium = "159.0.8154.34"

	releaseDate        = "2026-11-25"
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

	// The release this build replaces. It stays in the catalog so a profile
	// created on it moves here on its next open (same catalogue) instead of
	// being stranded on a build the pinned catalog no longer names.
	previousVersion      = "0.5.1.157"
	previousChromium     = "156.0.8078.74"
	previousDate         = "2026-11-07"
	previousZipName      = "anybrowser-156.0.8078.74-macos-arm64.zip"
	previousZipSHA256    = "acdc6ad19835ba6317254a00ff119bde73dc5d4857a82c7a0eca99cfeaea26e7"
	previousBinarySHA256 = "da649b02d3daed2cd2b5785e91ca9aaf60c4e727036aa756415adeda80736849"
	previousFrameworkSHA = "28e9691caf659331de5a617b67519ce7f1f418e7153dff3b81cdd9ca572e2933"
	previousRootDir      = "anybrowser-156.0.8078.74-macos-arm64"
	previousAppRel       = previousRootDir + "/Chromium.app"
	previousBinRel       = previousAppRel + "/Contents/MacOS/Chromium"
	previousFrameworkRel = previousAppRel + "/Contents/Frameworks/Chromium Framework.framework/Versions/156.0.8078.74/Chromium Framework"

	zipName = "anybrowser-159.0.8154.34-macos-arm64.zip"
	// engineTag names the pre-release the archive is attached to.
	engineTag     = "engine-anybrowser-0.5.1.159"
	defaultZipURL = "https://github.com/D0bby999/anyprofile/releases/download/" + engineTag + "/" + zipName
	zipSHA256     = "46631f041e9646236e951c556423f9c80fea9e5bbc03320aba82605e8d5f6eb4"
	binarySHA256  = "2a0f401f171df73a0597cf5a63747d75eeba7fc1b596782af1666748e33e0793"
	frameworkSHA  = "18ee47b3fdd01efccda6983a0ca5904be885b011e3140cb386824190cf0091d5"
	// teamID is the Developer ID team the bundle's signature must name.
	teamID = "25C8SNXS6Q"

	rootDir = "anybrowser-159.0.8154.34-macos-arm64"
	appRel  = rootDir + "/Chromium.app"
	binRel  = appRel + "/Contents/MacOS/Chromium"
	// frameworkRel holds the browser's code; the launcher binary is a stub.
	frameworkRel = appRel + "/Contents/Frameworks/Chromium Framework.framework/Versions/159.0.8154.34/Chromium Framework"
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
