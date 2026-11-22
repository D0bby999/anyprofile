// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"github.com/D0bby999/anyprofile/internal/service"
	"github.com/D0bby999/anyprofile/internal/store"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// PickAPKFile opens a native file chooser for a single .apk/.xapk/.apks/.apkm file.
func (a *App) PickAPKFile() (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: apkDialogTitle[a.svc.Language()],
		Filters: []runtime.FileFilter{
			{DisplayName: "Android apps (*.apk, *.apks, *.xapk, *.apkm)", Pattern: "*.apk;*.apks;*.xapk;*.apkm"},
		},
	})
}

func (a *App) ListAPKs() ([]store.MobileAPK, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.ListAPKs()
}

func (a *App) InspectAPK(path string) (service.APKInspect, error) {
	if err := a.ready(); err != nil {
		return service.APKInspect{}, err
	}
	return a.svc.InspectAPK(a.ctx, path)
}

func (a *App) AddAPK(in service.APKInput) (store.MobileAPK, error) {
	if err := a.ready(); err != nil {
		return store.MobileAPK{}, err
	}
	return a.svc.AddAPK(a.ctx, in)
}

func (a *App) LaunchAPK(pkg, deviceID string) error {
	return a.must(func() error { return a.svc.LaunchAPK(a.ctx, pkg, deviceID) })
}

func (a *App) SetAPKOrientation(pkg, orientation string) error {
	return a.must(func() error { return a.svc.SetAPKOrientation(pkg, orientation) })
}

func (a *App) UpdateAPK(in service.APKUpdate) (store.MobileAPK, error) {
	if err := a.ready(); err != nil {
		return store.MobileAPK{}, err
	}
	return a.svc.UpdateAPK(a.ctx, in)
}

func (a *App) SetAPKDevice(pkg, deviceID string) error {
	return a.must(func() error { return a.svc.SetAPKDevice(pkg, deviceID) })
}

func (a *App) CreateAPKShortcut(pkg string) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	return a.svc.CreateAPKShortcut(pkg)
}

func (a *App) DeleteAPK(pkg string, uninstall bool) error {
	return a.must(func() error { return a.svc.DeleteAPK(a.ctx, pkg, uninstall) })
}
