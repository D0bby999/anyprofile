// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build dev

package main

import (
	"path/filepath"
	"runtime"
)

// `wails dev` runs build/bin/AnyProfile.app/Contents/MacOS/AnyProfile on
// macOS and build/bin/AnyProfile-dev.exe on Windows; the helper is built in
// the repo by `make fury-config`.
func devConfigCandidates(exeDir string) []string {
	if runtime.GOOS == "windows" {
		return []string{filepath.Join(exeDir, "../../tools/fury-config/target/release/fury-config.exe")}
	}
	return []string{filepath.Join(exeDir, "../../../../../tools/fury-config/target/release/fury-config")}
}
