// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/engine/fury"
	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/netproxy"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/service"
	"github.com/D0bby999/anyprofile/internal/store"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App owns the services and is the only struct bound to the frontend.
// Its methods are thin: logic lives in package service.
type App struct {
	ctx      context.Context
	svc      *service.Service
	fury     *fury.Fury
	startErr error
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if err := a.init(); err != nil {
		a.startErr = err
	}
}

func (a *App) init() error {
	dirs, err := paths.Default()
	if err != nil {
		return err
	}
	if err := dirs.Ensure(); err != nil {
		return err
	}
	log := openLog(dirs)
	cfgBin, err := furyConfigPath()
	if err != nil {
		return err
	}
	st, err := store.Open(dirs.DB(), dirs.Backups())
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	a.fury = fury.New(dirs, cfgBin)
	sec := secrets.Keychain{}
	l := launcher.New(st, sec, dirs, []engine.Engine{a.fury},
		func(in launcher.Info) { runtime.EventsEmit(a.ctx, "launch:changed", in) }, log)
	a.svc = &service.Service{Dirs: dirs, Store: st, Secrets: sec, Engine: a.fury, Launcher: l, Log: log,
		CheckProxy: netproxy.Check,
		Locale: func(ctx context.Context, cc string) (string, []string, error) {
			loc, err := a.fury.LocaleFor(ctx, cc)
			return loc.UILocale, loc.Languages, err
		}}
	l.Adopt()
	return nil
}

// beforeClose closes every browser before quitting: proxy relays live in this
// process, so a browser left running would lose its network (it fails closed).
func (a *App) beforeClose(ctx context.Context) bool {
	if a.svc == nil {
		return false
	}
	n := len(a.svc.Launcher.List())
	if n == 0 {
		return false
	}
	choice, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         "Thoát AnyProfile?",
		Message:       fmt.Sprintf("Đang có %d trình duyệt mở. Thoát app sẽ đóng tất cả.", n),
		Buttons:       []string{"Đóng tất cả và thoát", "Huỷ"},
		DefaultButton: "Huỷ",
		CancelButton:  "Huỷ",
	})
	if err != nil || choice != "Đóng tất cả và thoát" {
		return true
	}
	a.svc.Launcher.StopAll()
	return false
}

// furyConfigPath finds the helper by absolute path inside the app bundle
// (Contents/Resources); dev builds also look in the repo. Never via PATH.
func furyConfigPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, _ = filepath.EvalSymlinks(exe)
	dir := filepath.Dir(exe)
	for _, p := range append([]string{filepath.Join(dir, "../Resources/fury-config")}, devConfigCandidates(dir)...) {
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
			return filepath.Clean(p), nil
		}
	}
	return "", errors.New("fury-config helper is missing from the app bundle; rebuild with make build")
}

// openLog writes to logs/anyprofile.log, keeping one previous file past 5 MB.
// Nothing secret is ever logged; proxies appear as user@host:port.
func openLog(dirs paths.Dirs) *slog.Logger {
	path := filepath.Join(dirs.Logs(), "anyprofile.log")
	if fi, err := os.Stat(path); err == nil && fi.Size() > 5<<20 {
		os.Rename(path, path+".1")
	}
	var w io.Writer = io.Discard
	if f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
		w = f
	}
	return slog.New(slog.NewTextHandler(w, nil))
}

// StartupError is shown instead of the app when initialization failed.
func (a *App) StartupError() string {
	if a.startErr == nil {
		return ""
	}
	return a.startErr.Error()
}

func (a *App) ready() error {
	if a.svc == nil {
		return errors.New("AnyProfile failed to start")
	}
	return nil
}
