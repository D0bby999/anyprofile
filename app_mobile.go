// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/macro"
	mobilesync "github.com/D0bby999/anyprofile/internal/mobile/sync"
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

var apkDialogTitle = map[string]string{"en": "Install apps (.apk, .apks, .xapk, .apkm)", "vi": "Cài app (.apk, .apks, .xapk, .apkm)"}

// InstallMobileApps asks for APK or split bundle files and installs them on a
// running device. Cancelling installs nothing and returns 0.
func (a *App) InstallMobileApps(id string) (int, error) {
	if err := a.ready(); err != nil {
		return 0, err
	}
	files, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{Title: apkDialogTitle[a.svc.Language()],
		Filters: []runtime.FileFilter{{DisplayName: "Android apps (*.apk, *.apks, *.xapk, *.apkm)", Pattern: "*.apk;*.apks;*.xapk;*.apkm"}}})
	if err != nil || len(files) == 0 {
		return 0, err
	}
	n, err := a.svc.InstallMobileApps(a.ctx, id, files)
	return n, err
}

// InstallMobileAurora downloads and installs Aurora Store on a running device.
func (a *App) InstallMobileAurora(id string) error {
	return a.must(func() error { return a.svc.InstallMobileAurora(a.ctx, id) })
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

// MobileWindowMuted reports whether the device's scrcpy audio is muted.
func (a *App) MobileWindowMuted(id string) (bool, error) {
	if err := a.ready(); err != nil {
		return false, err
	}
	return a.svc.MobileWindowMuted(id)
}

// SetMobileWindowMuted toggles scrcpy audio and reopens an open window.
func (a *App) SetMobileWindowMuted(id string, muted bool) error {
	return a.must(func() error { return a.svc.SetMobileWindowMuted(a.ctx, id, muted) })
}

var mediaDialogTitle = map[string]string{"en": "Push photos or videos to Gallery", "vi": "Đẩy ảnh/video vào Thư viện máy"}

// PushMobileMedia lets the user pick photos/videos and pushes them to the device Gallery.
func (a *App) PushMobileMedia(id string) (int, error) {
	if err := a.ready(); err != nil {
		return 0, err
	}
	files, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: mediaDialogTitle[a.svc.Language()],
		Filters: []runtime.FileFilter{{
			DisplayName: "Photos & Videos (*.mp4, *.mov, *.jpg, *.png, *.webp)",
			Pattern:     "*.mp4;*.mov;*.mkv;*.webm;*.jpg;*.jpeg;*.png;*.webp;*.heic",
		}},
	})
	if err != nil || len(files) == 0 {
		return 0, err
	}
	return a.svc.PushMobileMedia(a.ctx, id, files)
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

// SwipeMobileDevice performs a touch swipe in screen coordinates.
func (a *App) SwipeMobileDevice(id string, x1, y1, x2, y2, ms int) error {
	return a.must(func() error { return a.svc.SwipeMobileDevice(a.ctx, id, x1, y1, x2, y2, ms) })
}

// RotateMobileDevice toggles portrait / landscape orientation.
func (a *App) RotateMobileDevice(id string) error {
	return a.must(func() error { return a.svc.RotateMobileDevice(a.ctx, id) })
}

// ArrangeMobileWindows tiles open phone windows ("row", "grid2x2", "grid3x2", "cascade").
func (a *App) ArrangeMobileWindows(layout string) (int, error) {
	if err := a.ready(); err != nil {
		return 0, err
	}
	return a.svc.ArrangeMobileWindows(a.ctx, layout)
}

// BulkStartMobileDevices starts selected devices (max 2 concurrently).
func (a *App) BulkStartMobileDevices(ids []string) (service.BulkMobileResult, error) {
	if err := a.ready(); err != nil {
		return service.BulkMobileResult{}, err
	}
	return a.svc.BulkStartMobileDevices(a.ctx, ids), nil
}

// BulkStopMobileDevices stops selected devices concurrently.
func (a *App) BulkStopMobileDevices(ids []string) (service.BulkMobileResult, error) {
	if err := a.ready(); err != nil {
		return service.BulkMobileResult{}, err
	}
	return a.svc.BulkStopMobileDevices(a.ctx, ids), nil
}

// BulkInstallMobileApps picks APK/bundle files once and installs them onto all selected running devices.
func (a *App) BulkInstallMobileApps(ids []string) (service.BulkMobileResult, error) {
	if err := a.ready(); err != nil {
		return service.BulkMobileResult{}, err
	}
	files, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   apkDialogTitle[a.svc.Language()],
		Filters: []runtime.FileFilter{{DisplayName: "Android apps (*.apk, *.apks, *.xapk, *.apkm)", Pattern: "*.apk;*.apks;*.xapk;*.apkm"}},
	})
	if err != nil || len(files) == 0 {
		return service.BulkMobileResult{}, err
	}
	return a.svc.BulkInstallMobileApps(a.ctx, ids, files)
}

// StartMobileSync starts multi-instance synchronization from masterID to followerIDs.
func (a *App) StartMobileSync(masterID string, followerIDs []string, jitter mobilesync.Jitter) (mobilesync.Status, error) {
	if err := a.ready(); err != nil {
		return mobilesync.Status{}, err
	}
	return a.svc.StartMobileSync(masterID, followerIDs, jitter)
}

func (a *App) StopMobileSync() (mobilesync.Status, error) {
	if err := a.ready(); err != nil {
		return mobilesync.Status{}, err
	}
	return a.svc.StopMobileSync(), nil
}

func (a *App) MobileSyncStatus() (mobilesync.Status, error) {
	if err := a.ready(); err != nil {
		return mobilesync.Status{}, err
	}
	return a.svc.MobileSyncStatus(), nil
}

func (a *App) StartMacroRecord(deviceID, name string) error {
	return a.must(func() error { return a.svc.StartMacroRecord(deviceID, name) })
}

func (a *App) StopMacroRecord() (macro.Script, error) {
	if err := a.ready(); err != nil {
		return macro.Script{}, err
	}
	return a.svc.StopMacroRecord()
}

func (a *App) MobileMacroRecording() (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	return a.svc.MobileMacroRecording(), nil
}

func (a *App) ListMobileMacros() ([]macro.Script, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.ListMobileMacros()
}

func (a *App) RunMobileMacro(macroID string, deviceIDs []string, opts macro.PlayOptions) error {
	return a.must(func() error { return a.svc.RunMobileMacro(a.ctx, macroID, deviceIDs, opts) })
}

func (a *App) StopMobileMacro() error {
	return a.must(func() error { a.svc.StopMobileMacro(); return nil })
}

func (a *App) DeleteMobileMacro(macroID string) error {
	return a.must(func() error { return a.svc.DeleteMobileMacro(macroID) })
}

func (a *App) SetMobileLocation(id string, lat, lon float64) error {
	return a.must(func() error { return a.svc.SetMobileLocation(a.ctx, id, lat, lon) })
}

func (a *App) ResetMobileLocationToProxy(id string) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	return a.svc.ResetMobileLocationToProxy(a.ctx, id)
}

func (a *App) OpenMobileSharedFolder(id string) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	return a.svc.OpenMobileSharedFolder(id)
}

func (a *App) SyncMobileSharedFolder(id, direction string) (int, error) {
	if err := a.ready(); err != nil {
		return 0, err
	}
	return a.svc.SyncMobileSharedFolder(a.ctx, id, direction)
}

// FixMobilePlayProtect clears Google Services (gsf/gms/vending) on a running
// device without touching TikTok or other installed apps.
func (a *App) FixMobilePlayProtect(id string) error {
	return a.must(func() error { return a.svc.FixMobilePlayProtect(a.ctx, id) })
}

// CloneMobileDevice creates 1..10 APFS copy-on-write clones of a stopped
// Golden Master device with fresh hardware identities and first-boot Android ID reset.
func (a *App) CloneMobileDevice(sourceID, baseName string, count int, resetGoogleIDs bool) ([]store.MobileDevice, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.CloneMobileDevice(a.ctx, sourceID, baseName, count, resetGoogleIDs)
}

// CompactMobileDisk trims caches/fstrim (running) or punches APFS sparse holes (stopped).
func (a *App) CompactMobileDisk(id string) (service.CompactDiskResult, error) {
	if err := a.ready(); err != nil {
		return service.CompactDiskResult{}, err
	}
	return a.svc.CompactMobileDisk(a.ctx, id)
}

// MobileHostStats returns live host RAM, CPU, and recommended extra VM capacity.
func (a *App) MobileHostStats() (mobile.HostStats, error) {
	if err := a.ready(); err != nil {
		return mobile.HostStats{}, err
	}
	return a.svc.MobileHostStats(), nil
}

// SetMobileEcoMode toggles Multi-instance Eco Mode (30/10 FPS, RAM/CPU cap).
func (a *App) SetMobileEcoMode(on bool) error {
	return a.must(func() error { return a.svc.SetMobileEcoMode(on) })
}

// SetMobileFocusAudio toggles automatic background phone muting on focus switch.
func (a *App) SetMobileFocusAudio(on bool) error {
	return a.must(func() error { return a.svc.SetMobileFocusAudio(on) })
}

// SetMobileHotkeysEnabled toggles macOS global mobile hotkeys (Cmd+Shift+A/M/B).
func (a *App) SetMobileHotkeysEnabled(on bool) error {
	return a.must(func() error { return a.svc.SetMobileHotkeysEnabled(a.ctx, on) })
}
