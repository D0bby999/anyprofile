// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package mumu12

import (
	"os"
	"path/filepath"
)

// findInstall looks for MuMuPlayer 12's shell folder containing MuMuManager.exe.
func findInstall() (string, error) {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Netease", "MuMuPlayerGlobal-12.0", "shell"),
		filepath.Join(os.Getenv("ProgramFiles"), "Netease", "MuMu Player 12", "shell"),
		`C:\Program Files\Netease\MuMuPlayerGlobal-12.0\shell`,
		`C:\Program Files\Netease\MuMu Player 12\shell`,
		`D:\Program Files\Netease\MuMuPlayerGlobal-12.0\shell`,
		`D:\Program Files\Netease\MuMu Player 12\shell`,
	}
	for _, dir := range candidates {
		if _, err := os.Stat(filepath.Join(dir, "MuMuManager.exe")); err == nil {
			return dir, nil
		}
	}
	return "", errMissing
}
