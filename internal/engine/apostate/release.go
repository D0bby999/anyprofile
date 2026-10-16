// SPDX-License-Identifier: AGPL-3.0-or-later

// Package apostate runs Apostate (github.com/heretic-tech/apostate), a patched
// Chromium driven by command-line flags, as an AnyProfile engine. AnyProfile
// only downloads and executes the official release; it never modifies or
// redistributes it.
package apostate

// The pinned release. Hashes were checked against the downloaded files and the
// release's .manifest.json on 2026-10-14 (macOS) and 2026-10-16 (Windows)
// (plans/reports/research-261014-0001-apostate-integration.md). Each OS's
// build is in release_<os>.go; both carry the same persona tables.
const (
	Version  = "0.5.1"
	Chromium = "155.0.8059.31"
	// Catalogue is the persona-table digest of --fingerprint-explain. A
	// release with another digest re-draws every seed's machine; see the
	// upgrade rule in plans/reports/research-261014-0001-apostate-integration.md.
	Catalogue = "f7520eaac2212793d0ed147a82bf52303e77a8b18b91eb9e900c8e56acf0d117"
)

// HostPlatform and HostPersona are the persona this build offers: the OS it
// runs on (release_<os>.go).
const (
	HostPlatform = platform
	HostPersona  = personaID
)
