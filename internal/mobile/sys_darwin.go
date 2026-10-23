// SPDX-License-Identifier: AGPL-3.0-or-later

package mobile

import "golang.org/x/sys/unix"

// FreeBytes is the space an unprivileged user can still write on dir's volume.
func FreeBytes(dir string) (int64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil
}

// TotalRAM is the computer's physical memory in bytes.
func TotalRAM() (int64, error) {
	n, err := unix.SysctlUint64("hw.memsize")
	return int64(n), err
}
