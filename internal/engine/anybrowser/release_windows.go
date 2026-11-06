// SPDX-License-Identifier: AGPL-3.0-or-later

package anybrowser

// Keep the working upstream Windows x64 release until the project's Windows
// fork exists and passes native acceptance. Pins match the pre-rename release;
// the archive SHA was rechecked against the public upstream manifest 2026-10-29.
// No profile IDs, seeds, directories or installed data are migrated here.
const (
	Version  = "0.5.1"
	Chromium = "155.0.8059.31"

	releaseDate        = "2026-09-30"
	legacyGoogleKeyEnv = true
	platform           = "windows"
	// hostPlat is the release's platform in the engine catalog.
	hostPlat = "windows-amd64"
	// teamID is empty: the build is not signed.
	teamID    = ""
	personaID = "anybrowser-windows"
	// setsTZ is false: the Windows C runtime reads TZ as a POSIX rule such as
	// "EST5EDT" and would misread an IANA name. --fingerprint-timezone is enough.
	setsTZ = false
	// supportsGeolocation is false until the Windows fork carries patch 0164.
	supportsGeolocation = false

	zipName      = "apostate-155.0.8059.31-windows-x64.zip"
	zipURL       = "https://github.com/heretic-tech/apostate/releases/download/v0.5.1/" + zipName
	zipSHA256    = "8e55e44fb7f58619461e1d42a947446dcc08a2b46493c420c8e62ae27ab9c7e5"
	binarySHA256 = "5b4e065110a8a24bb35448d2716d26427a91618b0ee1f4b33723c4d25722de67"
	frameworkSHA = "200e51e059e774e3f1674b7e143a769f111c9ba412b50303e025f2fe7dbc2360"

	rootDir = "apostate-155.0.8059.31-windows-x64"
	appRel  = rootDir
	binRel  = rootDir + "/chrome.exe"
	// frameworkRel holds the browser's code; chrome.exe is a stub.
	frameworkRel = rootDir + "/chrome.dll"
)
