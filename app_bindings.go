// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"time"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/engine/fury"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/service"
	"github.com/D0bby999/anyprofile/internal/site"
	"github.com/D0bby999/anyprofile/internal/store"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Engine

func (a *App) EngineStatus() (engine.Status, error) {
	if err := a.ready(); err != nil {
		return engine.Status{}, err
	}
	return a.fury.Status(a.ctx), nil
}

// InstallEngine downloads and verifies the core, emitting "engine:progress".
func (a *App) InstallEngine() error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.fury.Install(a.ctx, func(done, total int64) {
		runtime.EventsEmit(a.ctx, "engine:progress", map[string]int64{"done": done, "total": total})
	})
}

func (a *App) Personas() ([]fury.Persona, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.fury.Personas(a.ctx)
}

// Profiles

func (a *App) ListProfiles(f store.ProfileFilter) ([]service.ProfileRow, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.ListProfiles(f)
}

// Bulk

func (a *App) CreateProfiles(in service.BulkCreate) ([]store.Profile, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.CreateProfiles(a.ctx, in)
}

func (a *App) DuplicateProfile(in service.DuplicateInput) ([]store.Profile, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.DuplicateProfile(a.ctx, in)
}

func (a *App) SetSiteMany(ids []string, site string) error {
	return a.must(func() error { return a.svc.SetSiteMany(ids, site) })
}

func (a *App) AddTagMany(ids []string, tag string) error {
	return a.must(func() error { return a.svc.AddTagMany(ids, tag) })
}

func (a *App) AssignProxiesMany(ids []string, mode string) map[string]string {
	if a.svc == nil {
		return nil
	}
	return a.svc.AssignProxiesMany(a.ctx, ids, mode)
}

func (a *App) Defaults() service.Defaults {
	if a.svc == nil {
		return service.Defaults{Site: "other", Proxy: "auto"}
	}
	return a.svc.Defaults()
}

func (a *App) SetDefaults(d service.Defaults) error {
	return a.must(func() error { return a.svc.SetDefaults(d) })
}

func (a *App) QuickCreate(name string) (store.Profile, error) {
	if err := a.ready(); err != nil {
		return store.Profile{}, err
	}
	return a.svc.QuickCreate(a.ctx, name)
}

func (a *App) CreateProfile(in service.NewProfile) (store.Profile, error) {
	if err := a.ready(); err != nil {
		return store.Profile{}, err
	}
	return a.svc.CreateProfile(a.ctx, in)
}

func (a *App) UpdateProfile(p store.Profile) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.svc.UpdateProfile(a.ctx, p)
}

func (a *App) TrashProfile(id string) error { return a.must(func() error { return a.svc.Trash(id) }) }
func (a *App) RestoreProfile(id string) error {
	return a.must(func() error { return a.svc.Restore(id) })
}
func (a *App) PurgeProfile(id string) error { return a.must(func() error { return a.svc.Purge(id) }) }

func (a *App) StartProfile(id string) error {
	return a.must(func() error {
		a.svc.Log.Info("ui asked to start", "profile", id)
		err := a.svc.Launcher.Start(a.ctx, id)
		a.svc.Log.Info("start returned", "profile", id, "err", err)
		return err
	})
}

func (a *App) StopProfile(id string) error {
	return a.must(func() error {
		a.svc.Log.Info("ui asked to stop", "profile", id)
		err := a.svc.Launcher.Stop(id)
		a.svc.Log.Info("stop returned", "profile", id, "err", err)
		return err
	})
}

func (a *App) FocusProfile(id string) bool { return a.svc != nil && a.svc.Launcher.Focus(id) }

// StartProfiles and StopProfiles return per-profile error messages.
func (a *App) StartProfiles(ids []string) map[string]string {
	if a.svc == nil {
		return nil
	}
	return a.svc.StartMany(a.ctx, ids)
}

func (a *App) StopProfiles(ids []string) map[string]string {
	if a.svc == nil {
		return nil
	}
	return a.svc.StopMany(ids)
}

// Proxies

func (a *App) ListProxies() ([]service.ProxyRow, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.ListProxies()
}

func (a *App) ImportProxies(text string) (service.ImportResult, error) {
	if err := a.ready(); err != nil {
		return service.ImportResult{}, err
	}
	return a.svc.ImportProxies(a.ctx, text)
}

func (a *App) CheckProxy(id string) (store.Proxy, error) {
	if err := a.ready(); err != nil {
		return store.Proxy{}, err
	}
	return a.svc.CheckProxyByID(a.ctx, id)
}

func (a *App) CheckAllProxies() error {
	return a.must(func() error { return a.svc.CheckAllProxies(a.ctx) })
}

func (a *App) RenameProxy(id, label string) error {
	return a.must(func() error { return a.svc.RenameProxy(id, label) })
}

func (a *App) DeleteProxy(id string) error {
	return a.must(func() error { return a.svc.DeleteProxy(id) })
}

// Sites

func (a *App) Sites() ([]site.Site, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Sites()
}

func (a *App) CreateSite(in service.SiteInput) (site.Site, error) {
	if err := a.ready(); err != nil {
		return site.Site{}, err
	}
	return a.svc.CreateSite(in)
}

func (a *App) UpdateSite(id string, in service.SiteInput) error {
	return a.must(func() error { return a.svc.UpdateSite(id, in) })
}

func (a *App) DeleteSite(id string) error {
	return a.must(func() error { return a.svc.DeleteSite(id) })
}

// Storage

func (a *App) Storage() (service.StorageInfo, error) {
	if err := a.ready(); err != nil {
		return service.StorageInfo{}, err
	}
	return a.svc.Storage()
}

func (a *App) DeleteOrphans(ids []string) error {
	return a.must(func() error { return a.svc.DeleteOrphans(ids) })
}

func (a *App) ClearCache() (int64, error) {
	if err := a.ready(); err != nil {
		return 0, err
	}
	return a.svc.ClearCache()
}

// Account

func (a *App) SetAccountPassword(id, pw string) error {
	return a.must(func() error { return a.svc.SetAccountPassword(id, pw) })
}

func (a *App) ClearAccountPassword(id string) error {
	return a.must(func() error { return a.svc.ClearAccountPassword(id) })
}

// CopyAccountPassword puts the password on the clipboard without it ever
// reaching the web UI, and clears it after 90 s if it is still there.
func (a *App) CopyAccountPassword(id string) error {
	return a.must(func() error {
		pw, err := a.svc.AccountPassword(id)
		if err != nil {
			return err
		}
		text := string(pw)
		secrets.Wipe(pw)
		if err := runtime.ClipboardSetText(a.ctx, text); err != nil {
			return err
		}
		go func() {
			time.Sleep(90 * time.Second)
			if cur, err := runtime.ClipboardGetText(a.ctx); err == nil && cur == text {
				runtime.ClipboardSetText(a.ctx, "")
			}
		}()
		return nil
	})
}

func (a *App) SetTOTP(id, secret string) error {
	return a.must(func() error { return a.svc.SetTOTP(id, secret) })
}

func (a *App) ClearTOTP(id string) error { return a.must(func() error { return a.svc.ClearTOTP(id) }) }

func (a *App) TOTPCode(id string) (service.TOTPCode, error) {
	if err := a.ready(); err != nil {
		return service.TOTPCode{}, err
	}
	return a.svc.TOTPCode(id)
}

// System

func (a *App) FileVaultOn() bool { return service.FileVaultOn() }

// Settings

func (a *App) Language() string {
	if a.svc == nil {
		return "en"
	}
	return a.svc.Language()
}

func (a *App) SetLanguage(lang string) error {
	return a.must(func() error { return a.svc.SetLanguage(lang) })
}

func (a *App) must(fn func() error) error {
	if err := a.ready(); err != nil {
		return err
	}
	return fn()
}
