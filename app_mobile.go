// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"github.com/D0bby999/anyprofile/internal/service"
	"github.com/D0bby999/anyprofile/internal/store"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

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

// InstallMobileRuntime downloads the Android VM runtime and image, emitting
// "engine:progress" like an engine install.
func (a *App) InstallMobileRuntime() error {
	return a.must(func() error {
		return a.svc.InstallMobileRuntime(a.ctx, func(engine string, done, total int64) {
			runtime.EventsEmit(a.ctx, "engine:progress", map[string]any{"engine": engine, "done": done, "total": total})
		})
	})
}

var apkDialogTitle = map[string]string{"en": "Install apps (.apk)", "vi": "Cài app (.apk)"}

// InstallMobileApps asks for APK files and installs them on a running device.
// Cancelling installs nothing and returns 0.
func (a *App) InstallMobileApps(id string) (int, error) {
	if err := a.ready(); err != nil {
		return 0, err
	}
	files, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{Title: apkDialogTitle[a.svc.Language()],
		Filters: []runtime.FileFilter{{DisplayName: "Android apps (*.apk)", Pattern: "*.apk"}}})
	if err != nil || len(files) == 0 {
		return 0, err
	}
	n, err := a.svc.InstallMobileApps(a.ctx, id, files)
	return n, err
}

// ShowMobileDevice opens a running device's window again.
func (a *App) ShowMobileDevice(id string) error {
	return a.must(func() error { return a.svc.ShowMobileDevice(a.ctx, id) })
}

func (a *App) BackupMobileDevice(id string) error {
	return a.must(func() error { return a.svc.BackupMobileDevice(a.ctx, id) })
}

func (a *App) RestoreMobileBackup(id string) error {
	return a.must(func() error { return a.svc.RestoreMobileBackup(a.ctx, id) })
}
