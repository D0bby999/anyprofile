// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/D0bby999/anyprofile/internal/agent"
	"github.com/D0bby999/anyprofile/internal/apperr"
)

// fakeApp answers the profile-management tools without a database.
type fakeApp struct {
	status   AppStatus
	proxies  []ProxyInfo
	created  []AppCreate
	patches  []AppPatch
	trashed  []string
	purged   []string
	assigned [][2]string
	extCalls int
}

func (f *fakeApp) Status(context.Context) (AppStatus, error) {
	st := f.status
	st.Version = "" // the tool fills it from the build
	return st, nil
}

func (f *fakeApp) Proxies() ([]ProxyInfo, error) { return f.proxies, nil }

func (f *fakeApp) CheckProxy(_ context.Context, id string) (ProxyInfo, error) {
	for _, p := range f.proxies {
		if p.ID == id {
			return p, nil
		}
	}
	return ProxyInfo{}, apperr.New("proxy_not_found", "no such proxy").Never()
}

func (f *fakeApp) CreateProfiles(_ context.Context, in AppCreate) ([]AppProfile, error) {
	f.created = append(f.created, in)
	out := make([]AppProfile, 0, in.Count)
	for i := 1; i <= in.Count; i++ {
		out = append(out, AppProfile{ID: "p" + string(rune('0'+i)), Serial: i, Name: in.BaseName, Engine: "anybrowser", ProxyID: "px1", Sites: in.Sites, Tags: in.Tags})
	}
	return out, nil
}

func (f *fakeApp) PatchProfile(_ context.Context, ref string, p AppPatch) (AppProfile, error) {
	f.patches = append(f.patches, p)
	return AppProfile{ID: ref, Name: "Kênh", Engine: "anybrowser"}, nil
}

func (f *fakeApp) Trash(_ context.Context, ref string) error {
	f.trashed = append(f.trashed, ref)
	return nil
}

func (f *fakeApp) Restore(_ context.Context, ref string) (AppProfile, error) {
	return AppProfile{ID: ref, Name: "Kênh", Engine: "anybrowser"}, nil
}

func (f *fakeApp) Purge(_ context.Context, ref, confirm string) error {
	if confirm != "Kênh" {
		return apperr.New("confirm_required", "confirm must be the profile's exact name").Never()
	}
	f.purged = append(f.purged, ref)
	return nil
}

func (f *fakeApp) AssignProxy(_ context.Context, ref, proxyID string) (AppProfile, error) {
	f.assigned = append(f.assigned, [2]string{ref, proxyID})
	return AppProfile{ID: ref, Name: "Kênh", Engine: "anybrowser", ProxyID: proxyID}, nil
}

func (f *fakeApp) SetExtension(_ context.Context, ids []string, _ string, _ bool) (int, error) {
	f.extCalls++
	return len(ids), nil
}

// appSetup builds an endpoint with the profile tools, as the app does when
// the user turned them on.
func appSetup(t *testing.T, app AppBackend, manage bool) *mcp.ClientSession {
	t.Helper()
	f := &fake{pipe: &pipe{done: make(chan struct{})}, profiles: []Profile{
		{ID: "p1", Serial: 1, Name: "Kênh", State: "stopped"},
	}}
	s := New(f, slog.New(slog.NewTextHandler(&logBuf, nil)), port, token)
	s.Agent = agent.NewHub(time.Minute)
	t.Cleanup(s.Agent.Close)
	s.Uploads, s.Recipes = t.TempDir(), t.TempDir()
	s.App, s.AgentManage = app, manage
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	cs, err := connect(t, srv, rewrite{token: token})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func toolNames(t *testing.T, cs *mcp.ClientSession) []string {
	t.Helper()
	list, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(list.Tools))
	for _, tl := range list.Tools {
		names = append(names, tl.Name)
	}
	return names
}

var appToolNames = []string{
	"app_status", "extension_set", "profile_create", "profile_purge", "profile_restore",
	"profile_trash", "profile_update", "proxy_assign", "proxy_check", "proxy_list",
}

// A tool that does not exist cannot be called: the switch is the whole gate.
func TestAppToolsOnlyWhenTheUserTurnedThemOn(t *testing.T) {
	off := toolNames(t, appSetup(t, &fakeApp{}, false))
	for _, name := range appToolNames {
		if slices.Contains(off, name) {
			t.Fatalf("%s exists with agent management off: %v", name, off)
		}
	}
	if len(off) != 30 {
		t.Fatalf("tools with the switch off = %d, want the 30 browser tools", len(off))
	}
	on := toolNames(t, appSetup(t, &fakeApp{}, true))
	for _, name := range appToolNames {
		if !slices.Contains(on, name) {
			t.Fatalf("%s missing when on: %v", name, on)
		}
	}
	if len(on) != 40 {
		t.Fatalf("tools with the switch on = %d, want 40", len(on))
	}
}

// The client is told which tools can destroy something, so it can ask first.
func TestAppToolsAnnotations(t *testing.T) {
	cs := appSetup(t, &fakeApp{}, true)
	list, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]*mcp.Tool{}
	for _, tl := range list.Tools {
		by[tl.Name] = tl
	}
	for _, name := range []string{"profile_trash", "profile_purge"} {
		a := by[name].Annotations
		if a == nil || a.DestructiveHint == nil || !*a.DestructiveHint {
			t.Fatalf("%s is not marked destructive: %+v", name, a)
		}
	}
	if a := by["app_status"].Annotations; a == nil || a.ReadOnlyHint == false {
		t.Fatalf("app_status must be read-only: %+v", a)
	}
	if a := by["profile_create"].Annotations; a == nil || a.DestructiveHint == nil || *a.DestructiveHint {
		t.Fatalf("profile_create must not be destructive: %+v", a)
	}
	// The embedded profile field reaches the schema, while only it is required.
	b, _ := json.Marshal(by["profile_update"].InputSchema)
	var sc struct {
		Properties map[string]any `json:"properties"`
		Required   []string       `json:"required"`
	}
	if err := json.Unmarshal(b, &sc); err != nil {
		t.Fatal(err)
	}
	if sc.Properties["profile"] == nil || sc.Properties["name"] == nil || sc.Properties["proxyId"] == nil {
		t.Fatalf("profile_update schema = %s", b)
	}
	if !slices.Equal(sc.Required, []string{"profile"}) {
		t.Fatalf("profile_update required = %v", sc.Required)
	}
}

func TestProfileTrashNeedsConfirm(t *testing.T) {
	app := &fakeApp{}
	cs := appSetup(t, app, true)
	// confirm is required by the schema, and false still has to be refused.
	if out, isErr := call(t, cs, "profile_trash", map[string]any{"profile": "Kênh"}); !isErr {
		t.Fatalf("trash without confirm was accepted: %q", out)
	}
	out, isErr := call(t, cs, "profile_trash", map[string]any{"profile": "Kênh", "confirm": false})
	if !isErr || !strings.Contains(out, "confirm_required") {
		t.Fatalf("trash with confirm=false = %q, err=%v", out, isErr)
	}
	if len(app.trashed) != 0 {
		t.Fatal("the profile was trashed without confirm")
	}
	if _, isErr := call(t, cs, "profile_trash", map[string]any{"profile": "Kênh", "confirm": true}); isErr {
		t.Fatal("trash with confirm failed")
	}
	if !slices.Equal(app.trashed, []string{"Kênh"}) {
		t.Fatalf("trashed = %v", app.trashed)
	}
}

func TestProfilePurgeNeedsTheExactName(t *testing.T) {
	app := &fakeApp{}
	cs := appSetup(t, app, true)
	if out, isErr := call(t, cs, "profile_purge", map[string]any{"profile": "Kênh", "confirm": "other"}); !isErr || !strings.Contains(out, "confirm_required") {
		t.Fatalf("purge with the wrong name = %q, err=%v", out, isErr)
	}
	if len(app.purged) != 0 {
		t.Fatal("purged with the wrong confirm")
	}
	if _, isErr := call(t, cs, "profile_purge", map[string]any{"profile": "Kênh", "confirm": "Kênh"}); isErr {
		t.Fatal("purge with the name failed")
	}
}

// What these tools answer is the shape of a profile, never its identity seed
// or the proxy's username.
func TestAppToolsCarryNoSecrets(t *testing.T) {
	app := &fakeApp{
		status:  AppStatus{Profiles: 3, Engines: []AppEngine{{ID: "anybrowser", Version: "0.5.1", Installed: true}}},
		proxies: []ProxyInfo{{ID: "px1", Label: "gw.example:10001", Scheme: "http", Host: "gw.example", Port: 10001, Country: "VN", Assignable: true}},
	}
	cs := appSetup(t, app, true)
	for _, name := range []string{"app_status", "proxy_list"} {
		out, isErr := call(t, cs, name, map[string]any{})
		if isErr {
			t.Fatalf("%s failed: %q", name, out)
		}
		for _, secret := range []string{"seed", "password", "username", "cookie"} {
			if strings.Contains(strings.ToLower(out), secret) {
				t.Fatalf("%s leaks %q: %q", name, secret, out)
			}
		}
	}
	out, _ := call(t, cs, "app_status", map[string]any{})
	if !strings.Contains(out, "version=") {
		t.Fatalf("app_status has no version: %q", out)
	}
	out, _ = call(t, cs, "profile_create", map[string]any{"baseName": "Kênh", "count": 2, "proxy": "auto"})
	if !strings.Contains(out, "Created 2") {
		t.Fatalf("create = %q", out)
	}
}
