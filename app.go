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
	"sort"
	"sync"
	"time"

	"github.com/D0bby999/anyprofile/internal/agent"
	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/engine/apostate"
	"github.com/D0bby999/anyprofile/internal/engine/fury"
	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/netproxy"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/service"
	"github.com/D0bby999/anyprofile/internal/store"
	"github.com/D0bby999/anyprofile/internal/update"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App owns the services and is the only struct bound to the frontend.
// Its methods are thin: logic lives in package service.
type App struct {
	ctx     context.Context
	svc     *service.Service
	engines map[string]engine.Engine
	// fury stays reachable on its own for LocaleFor and Personas: both use the
	// bundled fury-config helper and work before any engine is installed.
	fury     *fury.Fury
	startErr error
	api      apiState
	// agent is what the MCP endpoint knows of each browser; it lives as long
	// as the app, across restarts of the API server.
	agent *agent.Hub
	// backupFile is the backup chosen for a restore; the path stays in Go.
	backupMu   sync.Mutex
	backupFile string
	// proWait stops the wait for a purchase started in Settings → Pro.
	proMu   sync.Mutex
	proWait *purchaseWait
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
	relativeOverride := os.Getenv(paths.EnvDataDir) != "" && !filepath.IsAbs(os.Getenv(paths.EnvDataDir))
	if err := dirs.Ensure(); err != nil {
		return err
	}
	log := openLog(dirs)
	if relativeOverride {
		log.Warn("ignoring a relative data directory override", "env", paths.EnvDataDir)
	}
	cfgBin, err := furyConfigPath()
	if err != nil {
		return err
	}
	st, err := store.Open(dirs.DB(), dirs.Backups())
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	if err := st.BackupDaily(dirs.Backups(), time.Now()); err != nil {
		log.Warn("daily database backup", "err", err)
	}
	a.fury = fury.New(dirs, cfgBin)
	if err := a.fury.CleanStale(); err != nil {
		log.Warn("clean stale engine files", "err", err)
	}
	ap := apostate.New(dirs)
	if err := ap.CleanStale(); err != nil {
		log.Warn("clean stale engine files", "engine", "apostate", "err", err)
	}
	// One catalog for both: the compiled-in releases, and the signed list
	// from latest.json once LoadEngineList or an update check adds it.
	cat := engine.NewCatalog(update.Current, append(append(apostate.Builtin(), fury.Builtin()...), mobileBuiltin()...)...)
	ap.SetCatalog(cat)
	a.fury.SetCatalog(cat)
	a.engines = map[string]engine.Engine{"apostate": ap}
	if furyRuns {
		a.engines["fury"] = a.fury
	}
	// One Keychain item for every secret, read now: on macOS a new build
	// asks once, at launch, instead of per secret when a profile closes.
	// Secrets kept as their own items before move in now too.
	sec := secrets.NewVault(dirs.Vault(), secrets.Keychain{})
	go func() {
		if err := sec.Unlock(); err != nil {
			log.Warn("unlock secrets", "err", err)
		} else if err := sec.MoveIn(); err != nil {
			log.Warn("move secrets into the vault", "err", err)
		}
	}()
	l := launcher.New(st, sec, dirs, engineList(a.engines),
		func(in launcher.Info) { runtime.EventsEmit(a.ctx, "launch:changed", in) }, log)
	l.Capture = true
	l.CheckExit = netproxy.Check
	l.OnData = func(kind string) { runtime.EventsEmit(a.ctx, "data:changed", kind) }
	a.svc = &service.Service{Dirs: dirs, Store: st, Secrets: sec, Launcher: l, Log: log,
		Engines: a.engines, Preferred: "apostate", Catalog: cat,
		OnChange:   func(kind string) { runtime.EventsEmit(a.ctx, "data:changed", kind) },
		CheckProxy: netproxy.Check,
		Locale: func(ctx context.Context, cc string) (string, []string, error) {
			loc, err := a.fury.LocaleFor(ctx, cc)
			return loc.UILocale, loc.Languages, err
		}}
	if drv := mobileDriver(dirs, log, cat); drv != nil {
		a.svc.Mobile = mobile.NewManager(drv, st, log,
			func(in mobile.Info) { runtime.EventsEmit(a.ctx, "mobile:changed", in) })
	}
	l.BeforeStart, l.AfterExit = a.svc.BeforeLaunch, a.svc.AfterExit
	a.svc.LoadEngineList()
	go a.svc.PruneEngines(context.Background())
	l.Adopt()
	a.svc.AdoptMobileDevices(context.Background())
	a.agent = agent.NewHub(time.Minute)
	a.agent.Behind = l.Behind
	a.agent.OnActive = func(id string, on bool) {
		log.Info("agent", "profile", id, "driving", on)
		runtime.EventsEmit(a.ctx, "data:changed", "agent")
	}
	a.restartAPI()
	a.svc.CleanUpdates()
	a.setMenuLanguage(a.svc.Language())
	go a.svc.WatchMobileProxies(a.ctx)
	go func() {
		// Looked at every few hours, so an app left open for days still
		// learns of a release; CheckUpdate asks the server once a day at
		// most, and only when automatic checks are on. A release found is
		// downloaded and verified, so updating is only a restart. The Pro
		// pack is checked the same way, and only when a key is entered.
		for {
			a.checkPro()
			if st, _ := a.svc.CheckUpdate(context.Background(), false); st.Available {
				if st.Auto && !st.Ready {
					a.prefetchUpdate()
				}
				runtime.EventsEmit(a.ctx, "data:changed", "update")
			}
			time.Sleep(6 * time.Hour)
		}
	}()
	return nil
}

// engineList returns the engines sorted by ID.
func engineList(m map[string]engine.Engine) []engine.Engine {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]engine.Engine, 0, len(ids))
	for _, id := range ids {
		out = append(out, m[id])
	}
	return out
}

// quitText is the native quit dialog; the web UI has its own catalogs.
var quitText = map[string]struct{ Title, Message, Confirm, Cancel string }{
	"en": {"Quit AnyProfile?", "%d browsers or mobile devices are open. Quitting closes all of them.", "Close all and quit", "Cancel"},
	"vi": {"Thoát AnyProfile?", "Đang có %d trình duyệt hoặc máy mobile mở. Thoát app sẽ đóng tất cả.", "Đóng tất cả và thoát", "Huỷ"},
}

// beforeClose closes every browser before quitting: proxy relays live in this
// process, so a browser left running would lose its network (it fails closed).
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	defer func() {
		if !prevent {
			launcher.PassFocus() // stay on this desktop as the app quits
		}
	}()
	if a.svc == nil {
		return false
	}
	n := len(a.svc.Launcher.List())
	if a.svc.Mobile != nil {
		n += a.svc.Mobile.Running()
	}
	if n == 0 {
		return false
	}
	q := quitText[a.svc.Language()]
	choice, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         q.Title,
		Message:       fmt.Sprintf(q.Message, n),
		Buttons:       []string{q.Confirm, q.Cancel},
		DefaultButton: q.Cancel,
		CancelButton:  q.Cancel,
	})
	if err != nil || choice != q.Confirm {
		return true
	}
	a.svc.Launcher.StopAll()
	a.svc.StopAllMobile(ctx)
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
	for _, p := range append([]string{filepath.Join(dir, bundledConfigRel)}, devConfigCandidates(dir)...) {
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
