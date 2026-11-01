// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/service"
	"github.com/D0bby999/anyprofile/internal/store"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Mobile devices

// MobileFastStart reports whether closing a device saves its RAM state for a
// fast reopen. On by default.
func (a *App) MobileFastStart() (bool, error) {
	if err := a.ready(); err != nil {
		return true, err
	}
	return a.svc.MobileFastStart()
}

// SetMobileFastStart turns the saved-RAM state on or off; off deletes the
// saved states (~2.5 GB each).
func (a *App) SetMobileFastStart(on bool) error {
	return a.must(func() error { return a.svc.SetMobileFastStart(on) })
}

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

// ResetMobileDevice returns a device to a clean state (soft or hard).
func (a *App) ResetMobileDevice(id, mode string) error {
	return a.must(func() error { return a.svc.ResetMobileDevice(a.ctx, id, mode) })
}

// MobileMetrics returns boot, action and reset timings for this app run.
func (a *App) MobileMetrics(id string) (mobile.Metrics, error) {
	if err := a.ready(); err != nil {
		return mobile.Metrics{}, err
	}
	return a.svc.MobileMetrics(id)
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

// OpenMobileRuntimeSource opens the AnyMobile runtime's complete source (GPL)
// in the default browser.
func (a *App) OpenMobileRuntimeSource() error {
	return a.must(func() error {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		u, err := a.svc.MobileRuntimeSourceURL(ctx)
		if err != nil {
			return err
		}
		runtime.BrowserOpenURL(a.ctx, u)
		return nil
	})
}

// InstallMobileRuntime downloads the AnyMobile runtime and image, emitting
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

// MobilePreviewFrame returns the screen as base64 JPEG for the in-app viewer.
func (a *App) MobilePreviewFrame(id string) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	return a.svc.MobilePreviewFrame(a.ctx, id)
}

// TapMobileDevice taps a point in the device's 720x1600 screen space.
func (a *App) TapMobileDevice(id string, x int, y int) error {
	return a.must(func() error { return a.svc.TapMobileDevice(a.ctx, id, x, y) })
}

// PressMobileKey presses home, back or recents from the in-app viewer.
func (a *App) PressMobileKey(id string, key string) error {
	return a.must(func() error { return a.svc.PressMobileKey(a.ctx, id, key) })
}

// ScreenshotMobileDevice saves a JPEG of the Android screen and opens it.
func (a *App) ScreenshotMobileDevice(id string) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	return a.svc.ScreenshotMobileDevice(a.ctx, id)
}

// MobileWindowOnTop reports the stored always-on-top flag.
func (a *App) MobileWindowOnTop(id string) (bool, error) {
	if err := a.ready(); err != nil {
		return false, err
	}
	return a.svc.MobileWindowOnTop(id)
}

// SetMobileWindowOnTop stores the flag and reopens an open window.
func (a *App) SetMobileWindowOnTop(id string, on bool) error {
	return a.must(func() error { return a.svc.SetMobileWindowOnTop(a.ctx, id, on) })
}

// StartMobileRecording records the Android screen (max 3 minutes).
func (a *App) StartMobileRecording(id string) error {
	return a.must(func() error { return a.svc.StartMobileRecording(a.ctx, id) })
}

// StopMobileRecording stops, saves under ~/Movies/AnyProfile and opens it.
func (a *App) StopMobileRecording(id string) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	return a.svc.StopMobileRecording(a.ctx, id)
}

func (a *App) BackupMobileDevice(id string) error {
	return a.must(func() error { return a.svc.BackupMobileDevice(a.ctx, id) })
}

func (a *App) RestoreMobileBackup(id string) error {
	return a.must(func() error { return a.svc.RestoreMobileBackup(a.ctx, id) })
}
