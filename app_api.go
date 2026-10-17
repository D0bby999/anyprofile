// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"os"
	"os/exec"
	goruntime "runtime"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/D0bby999/anyprofile/internal/agent"
	"github.com/D0bby999/anyprofile/internal/api"
	"github.com/D0bby999/anyprofile/internal/service"
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
			State: string(r.State), DevToolsPort: r.CDPPort})
	}
	return out, nil
}

func (b apiBackend) Start(ctx context.Context, id string, devtools bool) error {
	b.svc.Log.Info("api asked to start", "profile", id, "devtools", devtools)
	return b.svc.StartProfile(ctx, id, devtools)
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

// restartAPI stops the server and starts it again when Settings has it on.
func (a *App) restartAPI() {
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
	srv.Uploads, srv.Recipes = a.svc.Dirs.Uploads(), a.svc.Dirs.Recipes()
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

// CopyMCPCommand copies the command that connects Claude Code to the MCP
// endpoint, token included; the clipboard is cleared after 90 s like the token.
func (a *App) CopyMCPCommand() error {
	return a.must(func() error {
		tok, err := a.svc.APIToken()
		if err != nil {
			return err
		}
		return copySecret(a.ctx, api.ClaudeCommand(a.svc.APISettings().Port, tok))
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
