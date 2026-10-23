// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"log/slog"

	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/vm"
	"github.com/D0bby999/anyprofile/internal/paths"
)

// mobileDriver: on macOS a device is a QEMU virtual machine.
func mobileDriver(dirs paths.Dirs, log *slog.Logger) mobile.Driver {
	return vm.New(dirs.MobileRuntime(), dirs.MobileDevices(), log)
}
