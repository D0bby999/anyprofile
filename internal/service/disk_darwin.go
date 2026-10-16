// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"os/exec"
	"strings"
)

// FileVaultOn reports whether the system volume is encrypted (FileVault on
// macOS, BitLocker on Windows). Profile data other than cookies (history,
// local storage) relies on it.
func FileVaultOn() bool {
	out, err := exec.Command("/usr/bin/fdesetup", "status").Output()
	return err == nil && strings.Contains(string(out), "FileVault is On")
}
