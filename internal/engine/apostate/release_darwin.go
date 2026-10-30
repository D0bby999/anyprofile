// SPDX-License-Identifier: AGPL-3.0-or-later

package apostate

// Official macOS arm64 release, verified against the upstream manifest on
// 2026-10-14. Preserve both the executable hashes and Developer ID team.
const (
	hostPlat = "darwin-arm64"
	teamID   = "PX3P7V373K"

	zipName      = "apostate-155.0.8059.31-macos-arm64.zip"
	zipURL       = "https://github.com/heretic-tech/apostate/releases/download/v0.5.1/" + zipName
	zipSHA256    = "58176ba9b3e9cfd74029165bdf716c636e618af17a48bd905fe6d5f1552d901a"
	binarySHA256 = "2dfa8b148e6d06598981c4a1e1bc0d7c2c7860d74401ad0d934d21a10c7916d7"
	frameworkSHA = "f2a43605e500acb24cb7384f2b85f276c5f055713f565ff11ee8fdf1f6e41dba"

	rootDir      = "apostate-155.0.8059.31-macos-arm64"
	appRel       = rootDir + "/Chromium.app"
	binRel       = appRel + "/Contents/MacOS/Chromium"
	frameworkRel = appRel + "/Contents/Frameworks/Chromium Framework.framework/Versions/155.0.8059.31/Chromium Framework"
)
