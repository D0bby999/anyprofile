// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/D0bby999/anyprofile/internal/agent"
	"github.com/D0bby999/anyprofile/internal/api"
	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/vm"
	"github.com/D0bby999/anyprofile/internal/pro"
	"github.com/D0bby999/anyprofile/internal/service"
	"github.com/D0bby999/anyprofile/internal/store"
	"strconv"
	"strings"
)

// apiState holds the running local API server, if any.
type apiState struct {
	mu  sync.Mutex
	srv *api.Server
	err string // why the last start failed, for Settings
}

// APIStatus is the Settings view of the local API.
type APIStatus struct {
	service.APISettings
	Running bool   `json:"running"`
	Error   string `json:"error"`
}

// apiBackend lets the API drive the service without knowing about Wails.
type apiBackend struct{ svc *service.Service }

func (b apiBackend) Profiles() ([]api.Profile, error) {
	rows, err := b.svc.LiveProfiles()
	if err != nil {
		return nil, err
	}
	out := make([]api.Profile, 0, len(rows))
	for _, r := range rows {
		out = append(out, api.Profile{ID: r.ID, Serial: r.Serial, Name: r.Name, Sites: r.Sites, Tags: r.Tags,
			State: string(r.State), Mode: string(r.Mode), DevToolsPort: r.CDPPort})
	}
	return out, nil
}

func (b apiBackend) Start(ctx context.Context, id string, devtools bool) error {
	b.svc.Log.Info("api asked to start", "profile", id, "devtools", devtools)
	return b.svc.StartProfile(ctx, id, devtools)
}

func (b apiBackend) StartHeadless(ctx context.Context, id string, devtools bool) error {
	b.svc.Log.Info("api asked to start", "profile", id, "devtools", devtools, "headless", true)
	return b.svc.StartProfileHeadless(ctx, id, devtools)
}

func (b apiBackend) Pipe(id string) (agent.Conn, bool) {
	c, ok := b.svc.Launcher.Conn(id)
	if !ok {
		return nil, false // a nil *cdppipe.Conn must not become a non-nil agent.Conn
	}
	return c, true
}

func (b apiBackend) Stop(id string) error {
	b.svc.Log.Info("api asked to stop", "profile", id)
	return b.svc.Launcher.Stop(id)
}

// apiMobile lets agents drive mobile devices through the service.
type apiMobile struct{ svc *service.Service }

func (m apiMobile) MobileDevices() ([]api.MobileDevice, error) {
	rows, err := m.svc.ListMobileDevices(store.ProfileFilter{})
	if err != nil {
		return nil, err
	}
	countries := map[string]string{}
	if pxs, err := m.svc.ListProxies(); err == nil {
		for _, p := range pxs {
			countries[p.ID] = p.Country
		}
	}
	out := make([]api.MobileDevice, 0, len(rows))
	for _, r := range rows {
		out = append(out, api.MobileDevice{ID: r.ID, Serial: r.Serial, Name: r.Name, State: string(r.State),
			Platform: r.Platform, Tags: r.Tags, Country: countries[r.ProxyID], Preset: r.Preset, ADBPort: r.ADBPort})
	}
	return out, nil
}

func (m apiMobile) StartMobile(ctx context.Context, id string) error {
	m.svc.Log.Info("api asked to start", "device", id, "headless", true)
	_, err := m.svc.StartMobileDeviceHeadless(ctx, id)
	return err
}

func (m apiMobile) StopMobile(ctx context.Context, id string) error {
	return m.svc.StopMobileDevice(ctx, id)
}

func (m apiMobile) ShowMobile(ctx context.Context, id string) error {
	return m.svc.ShowMobileDevice(ctx, id)
}

func (m apiMobile) ResetMobile(ctx context.Context, id, mode string) error {
	return m.svc.ResetMobileDevice(ctx, id, mode)
}

func (m apiMobile) MobileMetrics(id string) (mobile.Metrics, error) {
	return m.svc.MobileMetrics(id)
}

func (m apiMobile) MobileControl(id string) (api.DeviceControl, error) {
	c, err := m.svc.MobileControl(id)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (m apiMobile) proxyCountry(proxyID string) string {
	if proxyID == "" {
		return ""
	}
	if pxs, err := m.svc.ListProxies(); err == nil {
		for _, p := range pxs {
			if p.ID == proxyID {
				return p.Country
			}
		}
	}
	return ""
}

func (m apiMobile) toAPIDevice(d store.MobileDevice) api.MobileDevice {
	state := string(mobile.Stopped)
	adbPort := 0
	if m.svc.Mobile != nil {
		in := m.svc.Mobile.Info(d.ID)
		state, adbPort = string(in.State), in.ADBPort
	}
	return api.MobileDevice{
		ID: d.ID, Serial: d.Serial, Name: d.Name, State: state,
		Platform: d.Platform, Tags: d.Tags, Country: m.proxyCountry(d.ProxyID),
		Preset: d.Preset, ADBPort: adbPort,
	}
}

func (m apiMobile) resolveDevice(ref string, trashed bool) (store.MobileDevice, error) {
	rows, err := m.svc.ListMobileDevices(store.ProfileFilter{Trashed: trashed})
	if err != nil {
		return store.MobileDevice{}, err
	}
	ref = strings.TrimSpace(ref)
	serial, serr := strconv.Atoi(strings.TrimPrefix(ref, "#"))
	var hits []store.MobileDevice
	for _, r := range rows {
		if r.ID == ref || (serr == nil && r.Serial == serial) {
			return r.MobileDevice, nil
		}
		if strings.EqualFold(r.Name, ref) {
			hits = append(hits, r.MobileDevice)
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		where := "list_devices"
		if trashed {
			where = "the trash"
		}
		return store.MobileDevice{}, apperr.Newf("api_not_found", "no device %q in %s", ref, where)
	default:
		return store.MobileDevice{}, apperr.Newf("agent_ambiguous", "%d devices are named %q; use the id or #serial", len(hits), ref).Never()
	}
}

func (m apiMobile) CreateMobile(ctx context.Context, in api.MobileCreate) (api.MobileDevice, error) {
	proxyID := strings.TrimSpace(in.Proxy)
	if strings.EqualFold(proxyID, "none") {
		proxyID = ""
	}
	d, err := m.svc.CreateMobileDevice(ctx, service.MobileInput{
		Name: in.Name, Preset: in.Preset, ProxyID: proxyID,
		IdentityTemplate: in.IdentityTemplate, WithGMS: in.WithGMS,
		Tags: in.Tags, Notes: in.Notes,
	})
	if err != nil {
		return api.MobileDevice{}, err
	}
	return m.toAPIDevice(d), nil
}

func (m apiMobile) UpdateMobile(ctx context.Context, ref string, in api.MobilePatch) (api.MobileDevice, error) {
	cur, err := m.resolveDevice(ref, false)
	if err != nil {
		return api.MobileDevice{}, err
	}
	upd := service.MobileInput{
		Name: cur.Name, ProxyID: cur.ProxyID, Preset: cur.Preset,
		RAMMB: cur.RAMMB, CPUs: cur.CPUs, ScreenW: cur.ScreenW, ScreenH: cur.ScreenH, Density: cur.Density,
		Notes: cur.Notes, Tags: cur.Tags, Pinned: cur.Pinned,
	}
	if in.Name != nil {
		upd.Name = *in.Name
	}
	if in.Preset != nil && *in.Preset != cur.Preset {
		upd.Preset = *in.Preset
		upd.RAMMB, upd.CPUs, upd.ScreenW, upd.ScreenH, upd.Density = 0, 0, 0, 0, 0
	}
	if in.ProxyID != nil {
		upd.ProxyID = strings.TrimSpace(*in.ProxyID)
	}
	if in.IdentityTemplate != nil {
		upd.IdentityTemplate = *in.IdentityTemplate
	}
	if in.WithGMS != nil {
		upd.WithGMS = in.WithGMS
	}
	if in.Tags != nil {
		upd.Tags = *in.Tags
	}
	if in.Notes != nil {
		upd.Notes = *in.Notes
	}
	if err := m.svc.UpdateMobileDevice(ctx, cur.ID, upd); err != nil {
		return api.MobileDevice{}, err
	}
	got, err := m.svc.Store.GetMobileDevice(cur.ID)
	if err != nil {
		return api.MobileDevice{}, err
	}
	return m.toAPIDevice(got), nil
}

func (m apiMobile) AssignMobileProxy(ctx context.Context, ref, proxyID string) (api.MobileDevice, error) {
	return m.UpdateMobile(ctx, ref, api.MobilePatch{ProxyID: &proxyID})
}

func (m apiMobile) TrashMobile(ctx context.Context, ref string) error {
	d, err := m.resolveDevice(ref, false)
	if err != nil {
		return err
	}
	return m.svc.TrashMobileDevice(ctx, d.ID)
}

func (m apiMobile) RestoreMobile(_ context.Context, ref string) (api.MobileDevice, error) {
	d, err := m.resolveDevice(ref, true)
	if err != nil {
		return api.MobileDevice{}, err
	}
	if err := m.svc.RestoreMobileDevice(d.ID); err != nil {
		return api.MobileDevice{}, err
	}
	got, err := m.svc.Store.GetMobileDevice(d.ID)
	if err != nil {
		return api.MobileDevice{}, err
	}
	return m.toAPIDevice(got), nil
}

func (m apiMobile) PurgeMobile(ctx context.Context, ref, confirm string) error {
	d, err := m.resolveDevice(ref, true)
	if err != nil {
		return err
	}
	if strings.TrimSpace(confirm) != d.Name {
		return apperr.Newf("confirm_required", "confirm must be the device's exact name, %q", d.Name).Never()
	}
	return m.svc.PurgeMobileDevice(ctx, d.ID)
}

// apiAPK lets AI agents drive and manage standalone APKs through the service.
type apiAPK struct{ svc *service.Service }

func (a apiAPK) toAPIAPK(rec store.MobileAPK, countries map[string]string, running bool) api.APKApp {
	out := api.APKApp{
		Package: rec.Package, Label: rec.Label, VersionName: rec.VersionName,
		Orientation: rec.Orientation, ProxyID: rec.ProxyID, Country: countries[rec.ProxyID],
		Notes: rec.Notes, AndroidID: rec.AndroidID, Running: running,
	}
	if idn, ok := vm.ParseIdentity(rec.IdentityJSON); ok {
		out.Model, out.Template = idn.Model, idn.Template
	}
	return out
}

func (a apiAPK) proxyCountries() map[string]string {
	countries := map[string]string{}
	if pxs, err := a.svc.ListProxies(); err == nil {
		for _, p := range pxs {
			countries[p.ID] = p.Country
		}
	}
	return countries
}

func (a apiAPK) APKs() ([]api.APKApp, error) {
	list, err := a.svc.ListAPKs()
	if err != nil {
		return nil, err
	}
	countries := a.proxyCountries()
	running := a.svc.APKHostRunning()
	out := make([]api.APKApp, 0, len(list))
	for _, rec := range list {
		out = append(out, a.toAPIAPK(rec, countries, running))
	}
	return out, nil
}

func (a apiAPK) OpenAPK(ctx context.Context, pkg string, show bool) error {
	a.svc.Log.Info("api asked to open apk", "apk", pkg, "show", show)
	if show {
		return a.svc.LaunchAPK(ctx, pkg, "")
	}
	return a.svc.LaunchAPKHeadless(ctx, pkg)
}

func (a apiAPK) CloseAPK(ctx context.Context, pkg string, stopHost bool) error {
	return a.svc.CloseAPK(ctx, pkg, stopHost)
}

func (a apiAPK) ShowAPK(ctx context.Context, pkg string) error {
	return a.svc.ShowAPK(ctx, pkg)
}

func (a apiAPK) APKControl(pkg string) (api.DeviceControl, error) {
	return a.svc.APKControl(pkg)
}

func (a apiAPK) AddAPK(ctx context.Context, in api.APKCreate) (api.APKApp, error) {
	rec, err := a.svc.AddAPK(ctx, service.APKInput{
		FilePath: in.FilePath, Label: in.Label, ProxyID: in.ProxyID,
		Orientation: in.Orientation, IdentityTemplate: in.IdentityTemplate, Notes: in.Notes,
	})
	if err != nil {
		return api.APKApp{}, err
	}
	return a.toAPIAPK(rec, a.proxyCountries(), a.svc.APKHostRunning()), nil
}

func (a apiAPK) UpdateAPK(ctx context.Context, pkg string, in api.APKPatch) (api.APKApp, error) {
	cur, err := a.svc.Store.GetMobileAPK(pkg)
	if err != nil {
		return api.APKApp{}, err
	}
	upd := service.APKUpdate{
		Package: cur.Package, Label: cur.Label, Orientation: cur.Orientation,
		ProxyID: cur.ProxyID, Notes: cur.Notes,
	}
	if in.Label != nil {
		upd.Label = *in.Label
	}
	if in.Orientation != nil {
		upd.Orientation = *in.Orientation
	}
	if in.ProxyID != nil {
		upd.ProxyID = *in.ProxyID
	}
	if in.IdentityTemplate != nil {
		upd.IdentityTemplate = *in.IdentityTemplate
	}
	if in.Notes != nil {
		upd.Notes = *in.Notes
	}
	rec, err := a.svc.UpdateAPK(ctx, upd)
	if err != nil {
		return api.APKApp{}, err
	}
	return a.toAPIAPK(rec, a.proxyCountries(), a.svc.APKHostRunning()), nil
}

func (a apiAPK) DeleteAPK(ctx context.Context, pkg string, uninstall bool) error {
	return a.svc.DeleteAPK(ctx, pkg, uninstall)
}

// restartAPI stops the server and starts it again when Settings has it on.
func (a *App) restartAPI() {
	defer a.updateTray()
	a.api.mu.Lock()
	defer a.api.mu.Unlock()
	if a.api.srv != nil {
		a.api.srv.Close()
		a.api.srv = nil
	}
	a.api.err = ""
	set := a.svc.APISettings()
	if !set.Enabled {
		return
	}
	tok, err := a.svc.APIToken()
	if err != nil {
		a.api.err = err.Error()
		return
	}
	srv := api.New(apiBackend{a.svc}, a.svc.Log, set.Port, tok)
	srv.Agent = a.agent
	srv.App = apiBackend{a.svc}
	srv.AgentManage = set.AgentManage
	srv.Uploads, srv.Recipes = a.svc.Dirs.Uploads(), a.svc.Dirs.Recipes()
	srv.ProRecipes = pro.Recipes(a.svc.Dirs.Pro())
	srv.Traces = filepath.Join(a.svc.Dirs.Logs(), "traces")
	if a.svc.Mobile != nil {
		srv.Mobile = apiMobile{a.svc}
		srv.APK = apiAPK{a.svc}
	}
	if err := srv.Start(); err != nil {
		a.api.err = err.Error()
		a.svc.Log.Warn("api start", "err", err)
		return
	}
	a.api.srv = srv
}

// OpenAgentFolder shows the uploads or recipes folder in Finder or Explorer.
func (a *App) OpenAgentFolder(which string) error {
	return a.must(func() error {
		dir := a.svc.Dirs.Uploads()
		if which == "recipes" {
			dir = a.svc.Dirs.Recipes()
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		return revealFolder(dir)
	})
}

// AgentProfiles lists the profiles an AI agent is driving now.
func (a *App) AgentProfiles() ([]string, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.agent.Active(), nil
}

func (a *App) APIStatus() (APIStatus, error) {
	if err := a.ready(); err != nil {
		return APIStatus{}, err
	}
	a.api.mu.Lock()
	defer a.api.mu.Unlock()
	return APIStatus{APISettings: a.svc.APISettings(), Running: a.api.srv != nil, Error: a.api.err}, nil
}

func (a *App) SetAPISettings(s service.APISettings) (APIStatus, error) {
	if err := a.must(func() error { return a.svc.SetAPISettings(s) }); err != nil {
		return APIStatus{}, err
	}
	a.restartAPI()
	return a.APIStatus()
}

// CopyAPIToken puts the token on the clipboard; it never reaches the web UI.
func (a *App) CopyAPIToken() error {
	return a.must(func() error {
		tok, err := a.svc.APIToken()
		if err != nil {
			return err
		}
		return copySecret(a.ctx, tok)
	})
}

// CopyMCPSetup copies what connects client (an AI agent: claude, codex,
// gemini…) to the MCP endpoint, token included; the clipboard is cleared
// after 90 s like the token.
func (a *App) CopyMCPSetup(client string) error {
	return a.must(func() error {
		tok, err := a.svc.APIToken()
		if err != nil {
			return err
		}
		setup, err := api.MCPSetup(client, a.svc.APISettings().Port, tok)
		if err != nil {
			return err
		}
		return copySecret(a.ctx, setup)
	})
}

// NewAPIToken replaces the token, restarts the server with it and copies it.
func (a *App) NewAPIToken() error {
	return a.must(func() error {
		tok, err := a.svc.NewAPIToken()
		if err != nil {
			return err
		}
		a.restartAPI()
		return copySecret(a.ctx, tok)
	})
}

// copySecret puts text on the clipboard and clears it after 90 s if it is
// still there.
func copySecret(ctx context.Context, text string) error {
	if err := runtime.ClipboardSetText(ctx, text); err != nil {
		return err
	}
	go func() {
		time.Sleep(90 * time.Second)
		if cur, err := runtime.ClipboardGetText(ctx); err == nil && cur == text {
			runtime.ClipboardSetText(ctx, "")
		}
	}()
	return nil
}

func revealFolder(dir string) error {
	if goruntime.GOOS == "windows" {
		// explorer.exe exits 1 even when it opened the folder.
		exec.Command("explorer", dir).Run()
		return nil
	}
	return exec.Command("open", dir).Run()
}
