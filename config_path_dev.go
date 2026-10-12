// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build dev

package main

import "path/filepath"

// `wails dev` runs build/bin/AnyProfile.app/Contents/MacOS/AnyProfile; the
// helper is built in the repo by `make fury-config`.
func devConfigCandidates(exeDir string) []string {
	return []string{filepath.Join(exeDir, "../../../../../tools/fury-config/target/release/fury-config")}
}
