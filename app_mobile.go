// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"log/slog"

	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/service"
	"github.com/D0bby999/anyprofile/internal/store"
)

// mobileDriver is this OS's mobile device driver, nil while it has none:
// the QEMU VM driver (macOS) and the LDPlayer driver (Windows) plug in here.
func mobileDriver(dirs paths.Dirs, log *slog.Logger) mobile.Driver { return nil }

// Mobile devices

func (a *App) MobileAvailability() (service.MobileAvailability, error) {
	if err := a.ready(); err != nil {
		return service.MobileAvailability{}, err
	}
	return a.svc.MobileAvailability(), nil
}

func (a *App) ListMobileDevices(f store.ProfileFilter) ([]service.MobileRow, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.ListMobileDevices(f)
}

func (a *App) CreateMobileDevice(in service.MobileInput) (store.MobileDevice, error) {
	if err := a.ready(); err != nil {
		return store.MobileDevice{}, err
	}
	return a.svc.CreateMobileDevice(a.ctx, in)
}

func (a *App) UpdateMobileDevice(id string, in service.MobileInput) error {
	return a.must(func() error { return a.svc.UpdateMobileDevice(a.ctx, id, in) })
}

func (a *App) StartMobileDevice(id string) (service.MobileStart, error) {
	if err := a.ready(); err != nil {
		return service.MobileStart{}, err
	}
	a.svc.Log.Info("ui asked to start", "device", id)
	res, err := a.svc.StartMobileDevice(a.ctx, id)
	a.svc.Log.Info("start returned", "device", id, "err", err)
	return res, err
}

func (a *App) StopMobileDevice(id string) error {
	return a.must(func() error { return a.svc.StopMobileDevice(a.ctx, id) })
}

func (a *App) TrashMobileDevice(id string) error {
	return a.must(func() error { return a.svc.TrashMobileDevice(a.ctx, id) })
}

func (a *App) RestoreMobileDevice(id string) error {
	return a.must(func() error { return a.svc.RestoreMobileDevice(id) })
}

func (a *App) PurgeMobileDevice(id string) error {
	return a.must(func() error { return a.svc.PurgeMobileDevice(a.ctx, id) })
}

func (a *App) BackupMobileDevice(id string) error {
	return a.must(func() error { return a.svc.BackupMobileDevice(a.ctx, id) })
}

func (a *App) RestoreMobileBackup(id string) error {
	return a.must(func() error { return a.svc.RestoreMobileBackup(a.ctx, id) })
}
