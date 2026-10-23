// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"log/slog"

	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/paths"
)

// mobileDriver: on Windows a device is an LDPlayer instance; the driver
// comes with phase 7 of the mobile plan.
func mobileDriver(dirs paths.Dirs, log *slog.Logger) mobile.Driver { return nil }
