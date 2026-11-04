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
// The archive lives on its own engine pre-release, so it stays downloadable
// while /releases/latest keeps pointing at the newest app release (the
// updater reads latest.json from there). The bundle is signed with the same
// Developer ID as the app, which is what teamID below checks.
const (
	Version  = "0.5.1.156"
	Chromium = "156.0.8078.74"

	releaseDate        = "2026-11-04"
	legacyGoogleKeyEnv = false
	platform           = "macos"
	// hostPlat is the release's platform in the engine catalog.
	hostPlat  = "darwin-arm64"
	personaID = "anybrowser-macos"
	// setsTZ: the libc clock follows TZ, so the browser's own time code agrees
	// with --fingerprint-timezone.
	setsTZ = true

	zipName = "anybrowser-156.0.8078.74-macos-arm64.zip"
	// engineTag names the pre-release the archive is attached to.
	engineTag     = "engine-anybrowser-0.5.1.156"
	defaultZipURL = "https://github.com/D0bby999/anyprofile/releases/download/" + engineTag + "/" + zipName
	zipSHA256     = "3abbc0e039197ef0ef8bd95f9688b867636d8c75a2725c3e147cc60b2f5c2ee5"
	binarySHA256  = "efa45e35a42703e66fb24e2a4da02e9327fb8cba10e4941fb229f8564cafd097"
	frameworkSHA  = "6f8230d07923012d7fa337a028824f80d44eab48c30a58285a69f0474e7c29cf"
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
