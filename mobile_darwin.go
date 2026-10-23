// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"log/slog"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/vm"
	"github.com/D0bby999/anyprofile/internal/paths"
)

// mobileBuiltin is the pinned Android image; the runtime arrives in the signed list.
func mobileBuiltin() []engine.Release { return vm.ImageBuiltin() }

// mobileDriver: on macOS a device is a QEMU virtual machine.
func mobileDriver(dirs paths.Dirs, log *slog.Logger, cat *engine.Catalog) mobile.Driver {
	d := vm.New(dirs.MobileRuntime(), dirs.MobileDevices(), log)
	d.SetCatalog(cat)
	return d
}
