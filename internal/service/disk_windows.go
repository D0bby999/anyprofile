// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// FileVaultOn reports whether the system volume is encrypted (FileVault on
// macOS, BitLocker on Windows). Profile data other than cookies (history,
// local storage) relies on it.
//
// The shell's System.Volume.BitLockerProtection needs no administrator,
// unlike manage-bde: 1 is protection on, 6 on but locked; 2, 3 (encrypting),
// 4 (decrypting) and 5 (suspended) leave data readable.
func FileVaultOn() bool {
	drive := os.Getenv("SystemDrive")
	if drive == "" {
		drive = "C:"
	}
	script := "(New-Object -ComObject Shell.Application).NameSpace($env:SystemDrive + '\\').Self.ExtendedProperty('System.Volume.BitLockerProtection')"
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = append(os.Environ(), "SystemDrive="+drive)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	out, err := cmd.Output()
	v := strings.TrimSpace(string(out))
	return err == nil && (v == "1" || v == "6")
}
