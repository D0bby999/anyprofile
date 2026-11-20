// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"log/slog"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/avo"
	"github.com/D0bby999/anyprofile/internal/mobile/vm"
	"github.com/D0bby999/anyprofile/internal/paths"
)

// mobileBuiltin is the pinned Android image; the runtime comes with Pro (Service.OfferMobileRuntime).
func mobileBuiltin() []engine.Release { return vm.ImageBuiltin() }

// mobileDriver: on macOS prefers the Metal-accelerated Avo VMM when staged and keeps legacy QEMU VMs working.
func mobileDriver(dirs paths.Dirs, log *slog.Logger, cat *engine.Catalog) mobile.Driver {
	vmDrv := vm.New(dirs.MobileRuntime(), dirs.MobileDevices(), log)
	vmDrv.SetCatalog(cat)
	avoDrv := avo.New(dirs.MobileRuntime(), dirs.MobileDevices(), log)
	return avo.NewRouter(avoDrv, vmDrv)
}
