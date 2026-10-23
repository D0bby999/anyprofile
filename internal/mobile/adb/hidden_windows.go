// SPDX-License-Identifier: AGPL-3.0-or-later

package adb

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// hidden keeps a console window from flashing up for every adb call.
func hidden() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}
