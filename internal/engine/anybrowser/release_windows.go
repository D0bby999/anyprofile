// SPDX-License-Identifier: AGPL-3.0-or-later

package anybrowser

// The Windows x64 build of AnyBrowser, the project's own Chromium fork
// (pro/chrome). No Windows build exists yet — it needs a Windows build
// machine — so, as on macOS, zipURL is a placeholder and the hashes are
// zero: the publish step pins the real values and installing fails closed
// until then. A Windows host gets the Windows persona, whose fonts and GPU
// are the machine's own.
const (
	platform = "windows"
	// hostPlat is the release's platform in the engine catalog.
	hostPlat = "windows-amd64"
	// teamID is empty: the build is not signed.
	teamID    = ""
	personaID = "anybrowser-windows"
	// setsTZ is false: the Windows C runtime reads TZ as a POSIX rule such as
	// "EST5EDT" and would misread an IANA name. --fingerprint-timezone is enough.
	setsTZ = false

	zipName      = "anybrowser-156.0.8078.48-windows-x64.zip"
	zipURL       = "https://example.invalid/anybrowser/" + zipName
	zipSHA256    = "0000000000000000000000000000000000000000000000000000000000000000"
	binarySHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
	frameworkSHA = "0000000000000000000000000000000000000000000000000000000000000000"

	rootDir = "anybrowser-156.0.8078.48-windows-x64"
	appRel  = rootDir
	binRel  = rootDir + "/chrome.exe"
	// frameworkRel holds the browser's code; chrome.exe is a stub.
	frameworkRel = rootDir + "/chrome.dll"
)
