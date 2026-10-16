// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/D0bby999/anyprofile/internal/api"
	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/engine/apostate"
	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

// testBackend is the app's apiBackend (package main), rebuilt for the test.
type testBackend struct{ s *Service }

func (b testBackend) Profiles() ([]api.Profile, error) {
	rows, err := b.s.LiveProfiles()
	var out []api.Profile
	for _, r := range rows {
		out = append(out, api.Profile{ID: r.ID, Name: r.Name, State: string(r.State), DevToolsPort: r.CDPPort})
	}
	return out, err
}
func (b testBackend) Start(ctx context.Context, id string, devtools bool) error {
	return b.s.StartProfile(ctx, id, devtools)
}
func (b testBackend) Stop(id string) error { return b.s.Launcher.Stop(id) }

// A script opens a profile with DevTools through the API, drives it over
// the WebSocket the API returns, and closes it; the port is gone after.
// shortTemp keeps test data under /tmp on macOS: Chromium's SingletonSocket
// path must stay under the 104-byte limit of a Unix socket name.
func shortTemp() string {
	if runtime.GOOS == "darwin" {
		return "/tmp"
	}
	return ""
}

func TestAPIDrivesARealBrowser(t *testing.T) {
	app := os.Getenv("ANYPROFILE_TEST_APOSTATE_APP")
	if app == "" {
		t.Skip("ANYPROFILE_TEST_APOSTATE_APP not set")
	}
	root, _ := os.MkdirTemp(shortTemp(), "apapi")
	t.Cleanup(func() { os.RemoveAll(root) })
	dirs := paths.Dirs{Root: root}
	if err := dirs.Ensure(); err != nil {
		t.Fatal(err)
	}
	eng := apostate.New(dirs)
	os.MkdirAll(filepath.Dir(eng.AppPath()), 0o700)
	if err := os.Symlink(app, eng.AppPath()); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dirs.DB(), dirs.Backups())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	eng.MockKeychain = true
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	sec := secrets.NewMemory()
	l := launcher.New(st, sec, dirs, []engine.Engine{eng}, nil, log)
	l.Capture = true
	s := &Service{Dirs: dirs, Store: st, Secrets: sec, Log: log, Launcher: l,
		Engines: map[string]engine.Engine{"apostate": eng}, Preferred: "apostate"}
	if err := st.CreateProfile(&store.Profile{ID: "api1", Name: "API", Engine: "apostate",
		EngineVersion: apostate.Version, PersonaID: apostate.HostPersona, Platform: apostate.HostPlatform,
		SeedHex: "00000000000d0d0d", Timezone: "UTC", Languages: "en-US,en", UILocale: "en-US"}); err != nil {
		t.Fatal(err)
	}

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	tok, err := s.NewAPIToken()
	if err != nil {
		t.Fatal(err)
	}
	srv := api.New(testBackend{s}, log, port, tok)
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	call := func(method, path, body string) map[string]any {
		t.Helper()
		req, _ := http.NewRequest(method, fmt.Sprintf("http://127.0.0.1:%d%s", port, path), strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		json.NewDecoder(resp.Body).Decode(&out)
		if resp.StatusCode != 200 {
			t.Fatalf("%s %s = %d %v", method, path, resp.StatusCode, out)
		}
		return out
	}

	out := call("POST", "/v1/profiles/api1/start", `{"devtools": true}`)
	ws, _ := out["webSocketDebuggerUrl"].(string)
	dport, _ := out["devtoolsPort"].(float64)
	if out["state"] != "running" || ws == "" || dport == 0 {
		t.Fatalf("start = %v", out)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, ws, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", ws, err)
	}
	c.Write(ctx, websocket.MessageText, []byte(`{"id":1,"method":"Browser.getVersion"}`))
	_, msg, err := c.Read(ctx)
	c.Close(websocket.StatusNormalClosure, "")
	if err != nil || !strings.Contains(string(msg), "Chrome/155") {
		t.Fatalf("Browser.getVersion = %s, %v", msg, err)
	}

	out = call("POST", "/v1/profiles/api1/stop", "")
	if out["state"] != "stopped" {
		t.Fatalf("stop = %v", out)
	}
	if conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", int(dport)), time.Second); err == nil {
		conn.Close()
		t.Fatalf("DevTools port %d still open after stop", int(dport))
	}
	// The DevTools port was for that launch only.
	if p, _ := st.GetProfile("api1"); p.CDPNextLaunch {
		t.Fatal("cdp_next_launch left on")
	}
	srv.Close()
	if conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second); err == nil {
		conn.Close()
		t.Fatal("API still listening after Close")
	}
}
