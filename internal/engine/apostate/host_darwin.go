// SPDX-License-Identifier: AGPL-3.0-or-later

package apostate

import "golang.org/x/sys/unix"

func hostMemBytes() uint64 {
	mem, _ := unix.SysctlUint64("hw.memsize")
	return mem
}

// completeApp has nothing to add to the macOS release.
func completeApp(app, chromium string) error { return nil }
