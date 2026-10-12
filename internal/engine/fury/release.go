// SPDX-License-Identifier: AGPL-3.0-or-later

// Package fury runs the Fury core (github.com/furyteamtop/fury-antidetect-browser),
// a patched Chromium, as an AnyProfile engine. AnyProfile only downloads and
// executes the notarized core; it never modifies or redistributes it.
package fury

// The pinned release. Upgrading means changing every value here together with
// the fury-shared rev in tools/fury-config/Cargo.toml, then rerunning the
// integration tests. Hashes come from the release's SHA256SUMS and REPORT.md
// and were checked against the downloaded files on 2026-10-12.
const (
	Version  = "0.3.5"
	Chromium = "155.0.8059.39"
	// ChromeMajor and Chromium feed the derived config's UA and client hints.
	ChromeMajor = "155"

	tarballName   = "fury-core-0.3.5-macos-arm64.tar.xz"
	tarballURL    = "https://github.com/furyteamtop/fury-antidetect-browser/releases/download/v0.3.5/" + tarballName
	tarballSHA256 = "ac779d2eba94065f0ba649ef33be0a434bc1d0d237b8a772ca9290cc47c9c3c9"
	binarySHA256  = "bea5757de2048ef95cad9b656905ed3aae05faf6a831563d0228677a9437774b"
	teamID        = "JKN8WHX7FS"

	appName = "Fury.app"
	binRel  = "Fury.app/Contents/MacOS/Fury"
)
