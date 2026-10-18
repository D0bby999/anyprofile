// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/engine/fury"
	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/service"
	"github.com/D0bby999/anyprofile/internal/site"
	"github.com/D0bby999/anyprofile/internal/store"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"time"
)

// Engine

// EngineInfo is one engine's status plus whether new profiles use it.
type EngineInfo struct {
	engine.Status
	Default bool `json:"default"`
}

func (a *App) engine(id string) (engine.Engine, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if eng := a.engines[id]; eng != nil {
		return eng, nil
	}
	return nil, apperr.New("bad_engine", "unknown engine")
}

// Engines lists every engine, sorted by ID.
func (a *App) Engines() ([]EngineInfo, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	def := a.svc.DefaultEngine()
	var out []EngineInfo
	for _, eng := range engineList(a.engines) {
		out = append(out, EngineInfo{Status: a.svc.EngineState(a.ctx, eng), Default: eng.ID() == def})
	}
	return out, nil
}

func (a *App) EngineStatus(id string) (engine.Status, error) {
	eng, err := a.engine(id)
	if err != nil {
		return engine.Status{}, err
	}
	return a.svc.EngineState(a.ctx, eng), nil
}

// InstallEngine downloads and verifies an engine's newest release.
func (a *App) InstallEngine(id string) error {
	eng, err := a.engine(id)
	if err != nil {
		return err
	}
	rels := eng.Releases()
	if len(rels) == 0 {
		return apperr.New("bad_engine", "this engine has no release for this computer")
	}
	return a.InstallEngineVersion(id, rels[0].Version)
}

// InstallEngineVersion downloads and verifies one version, emitting
// "engine:progress".
func (a *App) InstallEngineVersion(id, version string) error {
	eng, err := a.engine(id)
	if err != nil {
		return err
	}
	err = eng.Install(a.ctx, version, func(done, total int64) {
		runtime.EventsEmit(a.ctx, "engine:progress", map[string]any{"engine": id, "version": version, "done": done, "total": total})
	})
	if err != nil {
		return err
	}
	a.svc.Log.Info("engine installed", "engine", id, "version", version)
	return nil
}

// EngineReleases lists an engine's versions, newest first.
func (a *App) EngineReleases(id string) ([]service.EngineRelease, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.EngineReleases(a.ctx, id)
}

// RemoveEngineVersion deletes an installed version; its profiles move to a
// newer one of the same catalogue.
func (a *App) RemoveEngineVersion(id, version string) error {
	return a.must(func() error { return a.svc.RemoveEngineVersion(a.ctx, id, version) })
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

func (a *App) SetSitesMany(ids []string, sites []string) error {
	return a.must(func() error { return a.svc.SetSitesMany(ids, sites) })
}

func (a *App) Sessions(profileID string) ([]store.Session, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Sessions(profileID)
}

// MatchExit moves the profile's timezone and languages to its proxy's current exit.
func (a *App) MatchExit(profileID string) error {
	return a.must(func() error { return a.svc.MatchExit(a.ctx, profileID) })
}

// Cookies. The file is chosen here, in Go: neither the path nor a cookie
// value passes through the web UI.

var cookieDialogTitle = map[string][2]string{
	"en": {"Import cookies", "Export cookies"},
	"vi": {"Nhập cookie", "Xuất cookie"},
}

func (a *App) ImportCookies(profileID string) (service.CookieImport, error) {
	if err := a.ready(); err != nil {
		return service.CookieImport{}, err
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   cookieDialogTitle[a.svc.Language()][0],
		Filters: []runtime.FileFilter{{DisplayName: "Cookies (*.json, *.txt)", Pattern: "*.json;*.txt"}},
	})
	if err != nil || path == "" {
		return service.CookieImport{}, err // cancelled: nothing to report
	}
	return a.svc.ImportCookies(a.ctx, profileID, path)
}

var extensionDialogTitle = map[string][3]string{
	"en": {"Add an extension (.zip or .crx)", "Add an unpacked extension folder", "Import bookmarks"},
	"vi": {"Thêm extension (.zip hoặc .crx)", "Thêm thư mục extension đã giải nén", "Nhập bookmark"},
}

// AddExtension asks for an extension (a folder when folder is true, else a
// .zip or .crx) and adds it to the library. Cancelling returns a zero value.
func (a *App) AddExtension(folder bool) (store.Extension, error) {
	if err := a.ready(); err != nil {
		return store.Extension{}, err
	}
	titles := extensionDialogTitle[a.svc.Language()]
	var path string
	var err error
	if folder {
		path, err = runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: titles[1]})
	} else {
		path, err = runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: titles[0],
			Filters: []runtime.FileFilter{{DisplayName: "Extensions (*.zip, *.crx)", Pattern: "*.zip;*.crx"}}})
	}
	if err != nil || path == "" {
		return store.Extension{}, err
	}
	return a.svc.AddExtension(path)
}

func (a *App) ListExtensions() ([]store.Extension, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.ListExtensions()
}

func (a *App) DeleteExtension(id string) error {
	return a.must(func() error { return a.svc.DeleteExtension(id) })
}

func (a *App) SetExtensionMany(ids []string, extID string, on bool) error {
	return a.must(func() error { return a.svc.SetExtensionMany(ids, extID, on) })
}

// ImportBookmarks asks for a bookmark HTML file and adds it to a closed
// profile; 0 with no error means the dialog was cancelled.
func (a *App) ImportBookmarks(profileID, folder string) (int, error) {
	if err := a.ready(); err != nil {
		return 0, err
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   extensionDialogTitle[a.svc.Language()][2],
		Filters: []runtime.FileFilter{{DisplayName: "Bookmarks (*.html, *.htm)", Pattern: "*.html;*.htm"}},
	})
	if err != nil || path == "" {
		return 0, err
	}
	return a.svc.ImportBookmarks(profileID, path, folder)
}

func (a *App) ExportCookies(profileID string) (int, error) {
	if err := a.ready(); err != nil {
		return 0, err
	}
	p, err := a.svc.Store.GetProfile(profileID)
	if err != nil {
		return 0, err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           cookieDialogTitle[a.svc.Language()][1],
		DefaultFilename: p.Name + "-cookies.json",
	})
	if err != nil || path == "" {
		return 0, err
	}
	return a.svc.ExportCookies(a.ctx, profileID, path)
}

// Backup of everything. The file is chosen here, in Go, like a cookie file.

var backupDialogTitle = map[string][2]string{
	"en": {"Save a backup of AnyProfile", "Restore from a backup"},
	"vi": {"Lưu bản sao lưu AnyProfile", "Khôi phục từ bản sao lưu"},
}

var backupFilter = []runtime.FileFilter{{DisplayName: "AnyProfile backup (*.apbackup)", Pattern: "*.apbackup"}}

// ExportBackup asks where to save, then writes the backup. A cancelled
// dialog returns an empty result.
func (a *App) ExportBackup(password string) (service.BackupResult, error) {
	if err := a.ready(); err != nil {
		return service.BackupResult{}, err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           backupDialogTitle[a.svc.Language()][0],
		DefaultFilename: "AnyProfile-" + time.Now().Format("2006-01-02") + ".apbackup",
		Filters:         backupFilter,
	})
	if err != nil || path == "" {
		return service.BackupResult{}, err
	}
	if filepath.Ext(path) == "" {
		path += ".apbackup"
	}
	return a.svc.ExportBackup(path, password)
}

// PickBackup asks for the backup to restore and returns its file name, ""
// when cancelled. ImportBackup then reads it with the password.
func (a *App) PickBackup() (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: backupDialogTitle[a.svc.Language()][1], Filters: backupFilter,
	})
	if err != nil || path == "" {
		return "", err
	}
	a.backupMu.Lock()
	a.backupFile = path
	a.backupMu.Unlock()
	return filepath.Base(path), nil
}

func (a *App) ImportBackup(password string) (service.BackupResult, error) {
	if err := a.ready(); err != nil {
		return service.BackupResult{}, err
	}
	a.backupMu.Lock()
	path := a.backupFile
	a.backupMu.Unlock()
	if path == "" {
		return service.BackupResult{}, apperr.New("backup_none", "choose a backup file first")
	}
	return a.svc.ImportBackup(a.ctx, path, password)
}

// Snapshots

func (a *App) Snapshots(profileID string) ([]store.Snapshot, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Snapshots(profileID)
}

func (a *App) TakeSnapshot(profileID string) (store.Snapshot, error) {
	if err := a.ready(); err != nil {
		return store.Snapshot{}, err
	}
	return a.svc.TakeSnapshot(profileID, "manual")
}

// RestoreSnapshot returns the id of the snapshot saved just before, for undo.
func (a *App) RestoreSnapshot(id int64) (int64, error) {
	if err := a.ready(); err != nil {
		return 0, err
	}
	return a.svc.RestoreSnapshot(id)
}

func (a *App) DeleteSnapshot(id int64) error {
	return a.must(func() error { return a.svc.DeleteSnapshot(id) })
}

func (a *App) DeleteAllSnapshots() error {
	return a.must(func() error { return a.svc.DeleteAllSnapshots() })
}

func (a *App) SnapshotSettings() service.SnapshotSettings {
	if a.svc == nil {
		return service.SnapshotSettings{}
	}
	return a.svc.SnapshotSettings()
}

func (a *App) SetSnapshotSettings(st service.SnapshotSettings) error {
	return a.must(func() error { return a.svc.SetSnapshotSettings(st) })
}

// Onboarded is false only on a fresh install that has not finished setup.
func (a *App) Onboarded() bool {
	if a.svc == nil {
		return true
	}
	return a.svc.Onboarded()
}

func (a *App) FinishOnboarding() error {
	return a.must(func() error { return a.svc.FinishOnboarding() })
}

func (a *App) SetPinnedMany(ids []string, pinned bool) error {
	return a.must(func() error { return a.svc.SetPinnedMany(ids, pinned) })
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

func (a *App) ResetFingerprint(id string) error {
	return a.must(func() error { return a.svc.ResetFingerprint(id) })
}
func (a *App) PurgeProfile(id string) error { return a.must(func() error { return a.svc.Purge(id) }) }

// StartProfile opens the browser behind the app, so several profiles can be
// opened in a row without each new window taking the focus; Focus shows it.
func (a *App) StartProfile(id string) error {
	return a.must(func() error {
		a.svc.Log.Info("ui asked to start", "profile", id)
		err := a.svc.Launcher.StartBehind(a.ctx, id)
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

// ArrangeWindows puts the open profiles' windows side by side on the screen
// AnyProfile is on. Only the UI offers it: the API and agents never resize.
func (a *App) ArrangeWindows() (int, error) {
	if err := a.ready(); err != nil {
		return 0, err
	}
	n, err := a.svc.Launcher.Arrange(a.ctx)
	if errors.Is(err, launcher.ErrNoScreen) {
		return 0, apperr.New("no_screen", "could not read the size of the screen")
	}
	return n, err
}

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

func (a *App) ImportProxies(text, expiresOn string) (service.ImportResult, error) {
	if err := a.ready(); err != nil {
		return service.ImportResult{}, err
	}
	return a.svc.ImportProxies(a.ctx, text, expiresOn)
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

func (a *App) UpdateProxy(id string, m service.ProxyMeta) error {
	return a.must(func() error { return a.svc.UpdateProxy(id, m) })
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

// Accounts: one per platform of a profile, named by the platform's site id.

func (a *App) SetAccountLogin(id, site, login string) error {
	return a.must(func() error { return a.svc.SetAccountLogin(id, site, login) })
}

func (a *App) SetAccountPassword(id, site, pw string) error {
	return a.must(func() error { return a.svc.SetAccountPassword(id, site, pw) })
}

func (a *App) ClearAccountPassword(id, site string) error {
	return a.must(func() error { return a.svc.ClearAccountPassword(id, site) })
}

func (a *App) ForgetAccount(id, site string) error {
	return a.must(func() error { return a.svc.ForgetAccount(id, site) })
}

// CopyAccountPassword puts the password on the clipboard without it ever
// reaching the web UI, and clears it after 90 s if it is still there.
func (a *App) CopyAccountPassword(id, site string) error {
	return a.must(func() error {
		pw, err := a.svc.AccountPassword(id, site)
		if err != nil {
			return err
		}
		text := string(pw)
		secrets.Wipe(pw)
		return copySecret(a.ctx, text)
	})
}

func (a *App) SetTOTP(id, site, secret string) error {
	return a.must(func() error { return a.svc.SetTOTP(id, site, secret) })
}

func (a *App) ClearTOTP(id, site string) error {
	return a.must(func() error { return a.svc.ClearTOTP(id, site) })
}

func (a *App) TOTPCode(id, site string) (service.TOTPCode, error) {
	if err := a.ready(); err != nil {
		return service.TOTPCode{}, err
	}
	return a.svc.TOTPCode(id, site)
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
	return a.must(func() error {
		if err := a.svc.SetLanguage(lang); err != nil {
			return err
		}
		a.setMenuLanguage(lang)
		return nil
	})
}

func (a *App) must(fn func() error) error {
	if err := a.ready(); err != nil {
		return err
	}
	return fn()
}

// Updates

func (a *App) UpdateStatus() (service.UpdateStatus, error) {
	if err := a.ready(); err != nil {
		return service.UpdateStatus{}, err
	}
	return a.svc.UpdateStatus(), nil
}

func (a *App) SetUpdateAuto(on bool) error {
	return a.must(func() error { return a.svc.SetUpdateAuto(on) })
}

func (a *App) CheckUpdate() (service.UpdateStatus, error) {
	if err := a.ready(); err != nil {
		return service.UpdateStatus{}, err
	}
	st, err := a.svc.CheckUpdate(a.ctx, true)
	if err == nil && st.Available && !st.Ready {
		go a.prefetchUpdate() // asked for: have it ready by the time Update is clicked
	}
	return st, err
}

func (a *App) prefetchUpdate() {
	if err := a.svc.PrefetchUpdate(context.Background()); err != nil {
		a.svc.Log.Warn("download update", "err", err)
	}
}

// TakeWhatsNew is the notes of the version just installed, once.
func (a *App) TakeWhatsNew() (service.WhatsNew, error) {
	if err := a.ready(); err != nil {
		return service.WhatsNew{}, err
	}
	return a.svc.TakeWhatsNew(), nil
}

// InstallUpdate downloads, verifies and installs the newest release, then
// quits; the installer starts the new version. Progress goes out as
// "update:progress" events with done and total bytes.
func (a *App) InstallUpdate() error {
	if err := a.ready(); err != nil {
		return err
	}
	var last int64
	err := a.svc.InstallUpdate(a.ctx, func(done, total int64) {
		if done-last >= 1<<20 || done == total {
			last = done
			runtime.EventsEmit(a.ctx, "update:progress", done, total)
		}
	})
	if err != nil {
		return err
	}
	go func() {
		time.Sleep(300 * time.Millisecond) // let the call return to the UI first
		runtime.Quit(a.ctx)
	}()
	return nil
}

// CopyDiagnostics puts a redacted report (versions, counts, end of the log)
// on the clipboard for a bug report. It holds no secret, so it is not
// cleared later the way a password is.
func (a *App) CopyDiagnostics() error {
	return a.must(func() error { return runtime.ClipboardSetText(a.ctx, a.svc.Diagnostics(a.ctx)) })
}
