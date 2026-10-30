// SPDX-License-Identifier: AGPL-3.0-or-later

package apostate

// Official Windows x64 release, verified against the upstream manifest on
// 2026-10-16 and rechecked on 2026-10-29. No Authenticode signature is present.
const (
	hostPlat = "windows-amd64"
	teamID   = ""

	zipName      = "apostate-155.0.8059.31-windows-x64.zip"
	zipURL       = "https://github.com/heretic-tech/apostate/releases/download/v0.5.1/" + zipName
	zipSHA256    = "8e55e44fb7f58619461e1d42a947446dcc08a2b46493c420c8e62ae27ab9c7e5"
	binarySHA256 = "5b4e065110a8a24bb35448d2716d26427a91618b0ee1f4b33723c4d25722de67"
	frameworkSHA = "200e51e059e774e3f1674b7e143a769f111c9ba412b50303e025f2fe7dbc2360"

	rootDir      = "apostate-155.0.8059.31-windows-x64"
	appRel       = rootDir
	binRel       = rootDir + "/chrome.exe"
	frameworkRel = rootDir + "/chrome.dll"
)
