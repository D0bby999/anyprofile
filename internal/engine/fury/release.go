// SPDX-License-Identifier: AGPL-3.0-or-later

// Package fury runs the Fury core (github.com/furyteamtop/fury-antidetect-browser),
// a patched Chromium, as an AnyProfile engine. AnyProfile only downloads and
// executes the notarized core; it never modifies or redistributes it.
package fury

import "github.com/D0bby999/anyprofile/internal/engine"

// Version is the newest compiled-in release. A core release that needs a new
// config format also needs the fury-shared rev in tools/fury-config/Cargo.toml
// moved with it, so it ships in an AnyProfile release; one that does not can
// be offered through the signed list (docs/releasing.md).
const Version = "0.4.5"

const (
	teamID  = "JKN8WHX7FS"
	appName = "Fury.app"
	binRel  = "Fury.app/Contents/MacOS/Fury"
	// catalogue: the machines come from the bundled fury-config, not the
	// core, so every core release keeps a profile's machine.
	catalogue = "fury-config"
)

// Builtin is every compiled-in core release, newest first. Hashes come from
// each release's SHA256SUMS and REPORT.md ("core sha256") and were checked
// against the downloaded files: 0.3.5 on 2026-10-12, 0.4.5 on 2026-10-17.
// 0.3.5 stays listed so profiles on it keep opening until 0.4.5 is installed.
func Builtin() []engine.Release {
	rel := func(version, date, tarSHA, binSHA string) engine.Release {
		return engine.Release{Engine: "fury", Version: version, Chromium: "155.0.8059.39", Platform: "darwin-arm64",
			Date: date, URL: "https://github.com/furyteamtop/fury-antidetect-browser/releases/download/v" + version +
				"/fury-core-" + version + "-macos-arm64.tar.xz",
			SHA256: tarSHA, Files: map[string]string{binRel: binSHA}, Bin: binRel, App: appName, TeamID: teamID,
			Catalogue: catalogue}
	}
	return []engine.Release{
		rel("0.4.5", "2026-10-15", "870332b01650523022969f29662ea08a09e0f0ff80d3385f92ea89c7907cb713",
			"4f36220da85fc0b351ec21addeec09766e4fd3f2402752635340cebe35844767"),
		rel("0.3.5", "2026-10-12", "ac779d2eba94065f0ba649ef33be0a434bc1d0d237b8a772ca9290cc47c9c3c9",
			"bea5757de2048ef95cad9b656905ed3aae05faf6a831563d0228677a9437774b"),
	}
}
