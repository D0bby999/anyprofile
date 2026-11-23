// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"io/fs"
	"os/exec"
	"strings"
	"syscall"
)

// fileDiskBytes returns the on-disk size for sparse files (such as 32 GiB
// Android userdata.img images) and the exact byte length for normal files.
func fileDiskBytes(info fs.FileInfo) int64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		if allocated := st.Blocks * 512; allocated < info.Size() {
			return allocated
		}
	}
	return info.Size()
}

// FileVaultOn reports whether the system volume is encrypted (FileVault on
// macOS, BitLocker on Windows). Profile data other than cookies (history,
// local storage) relies on it.
func FileVaultOn() bool {
	out, err := exec.Command("/usr/bin/fdesetup", "status").Output()
	return err == nil && strings.Contains(string(out), "FileVault is On")
}
