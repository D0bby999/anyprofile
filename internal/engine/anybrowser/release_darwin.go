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
// The first AnyBrowser build is not hosted yet: zipURL is a placeholder and
// the hashes are zero. The publish step (docs/releasing.md) pins the real
// URL and hashes; until then installing this release fails closed at
// download/verification instead of fetching the old upstream build.
const (
	Version  = "0.5.1.156"
	Chromium = "156.0.8078.74"

	releaseDate        = "2026-11-02"
	legacyGoogleKeyEnv = false
	platform           = "macos"
	// hostPlat is the release's platform in the engine catalog.
	hostPlat  = "darwin-arm64"
	personaID = "anybrowser-macos"
	// setsTZ: the libc clock follows TZ, so the browser's own time code agrees
	// with --fingerprint-timezone.
	setsTZ = true

	zipName       = "anybrowser-156.0.8078.74-macos-arm64.zip"
	defaultZipURL = "https://example.invalid/anybrowser/" + zipName
	zipSHA256     = "999d18efbec1e23005f43f95e7125bee44fcd8fdfa9dfcf4befb98dc1a2ec965"
	binarySHA256  = "77db438f4eb6f122439ac0fe8c79469a497e9e400b32ae23d4047cf9b3530b56"
	frameworkSHA  = "803d22f3dbd74261ec8ca1e45bc8ee38b0bf577e29ad6b960e9696baa5db69fc"
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
