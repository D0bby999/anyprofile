// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"log/slog"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/avo"
	"github.com/D0bby999/anyprofile/internal/paths"
)

// mobileBuiltin is empty on macOS: AnyMobile Metal pulls its runtime from Pro
// and its Android image directly from Google's public Cuttlefish Build API.
func mobileBuiltin() []engine.Release { return nil }

// mobileDriver returns the Metal-accelerated AnyMobile driver on macOS.
func mobileDriver(dirs paths.Dirs, log *slog.Logger, _ *engine.Catalog) mobile.Driver {
	return avo.New(dirs.MobileRuntime(), dirs.MobileDevices(), log)
}
