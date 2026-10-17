// SPDX-License-Identifier: AGPL-3.0-or-later

package apostate

// The Windows x64 build of the pinned release, checked 2026-10-16 against
// apostate-155.0.8059.31-windows-x64.zip.manifest.json. The build is not
// Authenticode-signed, so the pinned hashes are the only check. A Windows
// host gets the Windows persona, whose fonts and GPU are the machine's own.
const (
	platform = "windows"
	// hostPlat is the release's platform in the engine catalog.
	hostPlat = "windows-amd64"
	// teamID is empty: the build is not signed.
	teamID    = ""
	personaID = "apostate-windows"
	// setsTZ is false: the Windows C runtime reads TZ as a POSIX rule such as
	// "EST5EDT" and would misread an IANA name. --fingerprint-timezone is enough.
	setsTZ = false

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
