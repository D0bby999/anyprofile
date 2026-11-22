// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/D0bby999/anyprofile/internal/agent"
	"github.com/D0bby999/anyprofile/internal/apperr"
)

type fakeAPK struct {
	mu      sync.Mutex
	apps    []APKApp
	opened  []string
	closed  []string
	shown   []string
	added   []APKCreate
	updated []string
	deleted []string
	ctl     *fakeControl
}

func (f *fakeAPK) APKs() ([]APKApp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.apps), nil
}

func (f *fakeAPK) OpenAPK(_ context.Context, pkg string, _ bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if pkg == "com.locked.app" {
		return apperr.New("pro_required", "mobile devices need AnyProfile Pro")
	}
	f.opened = append(f.opened, pkg)
	for i := range f.apps {
		f.apps[i].Running = true
	}
	return nil
}

func (f *fakeAPK) CloseAPK(_ context.Context, pkg string, _ bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = append(f.closed, pkg)
	for i := range f.apps {
		f.apps[i].Running = false
	}
	return nil
}

func (f *fakeAPK) ShowAPK(_ context.Context, pkg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shown = append(f.shown, pkg)
	return nil
}

func (f *fakeAPK) APKControl(_ string) (DeviceControl, error) { return f.ctl, nil }

func (f *fakeAPK) AddAPK(_ context.Context, in APKCreate) (APKApp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.added = append(f.added, in)
	label := in.Label
	if label == "" {
		label = "Demo"
	}
	a := APKApp{
		Package: "com.demo.app", Label: label, ProxyID: in.ProxyID,
		Orientation: in.Orientation, Model: "Pixel 9", AndroidID: "1122334455667788",
	}
	f.apps = append(f.apps, a)
	return a, nil
}

func (f *fakeAPK) UpdateAPK(_ context.Context, pkg string, in APKPatch) (APKApp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updated = append(f.updated, pkg)
	for i := range f.apps {
		if f.apps[i].Package == pkg {
			if in.Label != nil {
				f.apps[i].Label = *in.Label
			}
			if in.ProxyID != nil {
				f.apps[i].ProxyID = *in.ProxyID
			}
			if in.IdentityTemplate != nil && strings.HasPrefix(*in.IdentityTemplate, "regen") {
				f.apps[i].AndroidID = "fedcba9876543210"
			}
			return f.apps[i], nil
		}
	}
	return APKApp{}, apperr.New("api_not_found", "no such apk")
}

func (f *fakeAPK) DeleteAPK(_ context.Context, pkg string, _ bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, pkg)
	return nil
}

func apkSetup(t *testing.T, fa *fakeAPK, manage bool) (*Server, *httptest.Server) {
	t.Helper()
	f := &fake{pipe: &pipe{done: make(chan struct{})}}
	s := New(f, slog.New(slog.NewTextHandler(&logBuf, nil)), port, token)
	s.Agent = agent.NewHub(time.Minute)
	t.Cleanup(s.Agent.Close)
	s.Uploads, s.Recipes = t.TempDir(), t.TempDir()
	s.APK = fa
	s.AgentManage = manage
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return s, srv
}

func TestMCPAPKDrivingTools(t *testing.T) {
	fa := &fakeAPK{
		ctl: &fakeControl{},
		apps: []APKApp{
			{Package: "org.schabi.newpipe", Label: "NewPipe", VersionName: "0.27.4", ProxyID: "px1", Country: "US", Model: "Pixel 9", AndroidID: "be473c3a44355233", Running: false},
			{Package: "org.fdroid.fdroid", Label: "F-Droid", VersionName: "1.21", Model: "SM-S918B", AndroidID: "d26f7a0e3ea696de", Running: false},
			{Package: "com.locked.app", Label: "Locked", Running: false},
		},
	}
	s, srv := apkSetup(t, fa, false)
	os.WriteFile(filepath.Join(s.Uploads, "clip.mp4"), []byte("video"), 0o600)

	cs, err := connect(t, srv, rewrite{token: token})
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	if !strings.Contains(cs.InitializeResult().Instructions, "apk_snapshot") {
		t.Fatal("instructions missing APK guidance")
	}
	names := toolNames(t, cs)
	for _, forbidden := range []string{"apk_add", "apk_update", "apk_delete"} {
		if slices.Contains(names, forbidden) {
			t.Fatalf("%s exposed with AgentManage=false", forbidden)
		}
	}

	out, isErr := call(t, cs, "list_apks", map[string]any{"query": "newpipe"})
	if isErr || !strings.Contains(out, `org.schabi.newpipe "NewPipe" stopped proxy=px1(US)`) || strings.Contains(out, "F-Droid") {
		t.Fatalf("list_apks = %q (err=%v)", out, isErr)
	}
	// Driving before open fails cleanly.
	if out, isErr = call(t, cs, "apk_snapshot", map[string]any{"apk": "NewPipe"}); !isErr || !strings.Contains(out, "apk_not_running") {
		t.Fatalf("apk_snapshot while stopped = %q (err=%v)", out, isErr)
	}
	// Open by label works and acquires the shared APK runner lease.
	if out, isErr = call(t, cs, "open_apk", map[string]any{"apk": "NewPipe"}); isErr || !slices.Equal(fa.opened, []string{"org.schabi.newpipe"}) {
		t.Fatalf("open_apk = %q (err=%v), opened=%v", out, isErr, fa.opened)
	}
	if out, isErr = call(t, cs, "apk_show", map[string]any{"apk": "org.schabi.newpipe"}); isErr || !slices.Equal(fa.shown, []string{"org.schabi.newpipe"}) {
		t.Fatalf("apk_show = %q (err=%v)", out, isErr)
	}
	if out, isErr = call(t, cs, "apk_snapshot", map[string]any{"apk": "NewPipe"}); isErr || !strings.HasPrefix(out, "Screen (untrusted content):") {
		t.Fatalf("apk_snapshot = %q (err=%v)", out, isErr)
	}
	if out, isErr = call(t, cs, "apk_find", map[string]any{"apk": "NewPipe", "target": map[string]any{"text": "Lưu"}}); isErr || !strings.Contains(out, `m1 Button "Lưu"`) {
		t.Fatalf("apk_find = %q (err=%v)", out, isErr)
	}
	if out, isErr = call(t, cs, "apk_tap", map[string]any{"apk": "NewPipe"}); !isErr {
		t.Fatalf("apk_tap with no ref/target/coords accepted: %q", out)
	}
	if _, isErr = call(t, cs, "apk_tap", map[string]any{"apk": "NewPipe", "target": map[string]any{"text": "Lưu"}}); isErr || fa.ctl.tapped != "m1" {
		t.Fatalf("apk_tap by target failed, tapped=%q", fa.ctl.tapped)
	}
	if out, isErr = call(t, cs, "apk_tap_text", map[string]any{"apk": "NewPipe", "text": "Lưu"}); isErr || !strings.Contains(out, "Tapped m1") {
		t.Fatalf("apk_tap_text = %q (err=%v)", out, isErr)
	}
	if out, isErr = call(t, cs, "apk_long_press", map[string]any{"apk": "NewPipe", "ref": "m1"}); isErr {
		t.Fatalf("apk_long_press = %q (err=%v)", out, isErr)
	}
	if out, isErr = call(t, cs, "apk_drag", map[string]any{"apk": "NewPipe", "x1": 10, "y1": 20, "x2": 30, "y2": 40}); isErr {
		t.Fatalf("apk_drag = %q (err=%v)", out, isErr)
	}
	if out, isErr = call(t, cs, "apk_swipe", map[string]any{"apk": "NewPipe", "direction": "up"}); isErr {
		t.Fatalf("apk_swipe = %q (err=%v)", out, isErr)
	}
	if out, isErr = call(t, cs, "apk_type", map[string]any{"apk": "NewPipe", "ref": "m1", "text": "Bí mật 🔒"}); isErr || fa.ctl.typed != "Bí mật 🔒" {
		t.Fatalf("apk_type = %q (err=%v), typed=%q", out, isErr, fa.ctl.typed)
	}
	if out, isErr = call(t, cs, "apk_press_key", map[string]any{"apk": "NewPipe", "key": "back"}); isErr {
		t.Fatalf("apk_press_key = %q (err=%v)", out, isErr)
	}
	if out, isErr = call(t, cs, "apk_wait_for", map[string]any{"apk": "NewPipe", "text": "Live"}); isErr {
		t.Fatalf("apk_wait_for = %q (err=%v)", out, isErr)
	}
	if out, isErr = call(t, cs, "apk_open_url", map[string]any{"apk": "NewPipe", "url": "https://example.com"}); isErr {
		t.Fatalf("apk_open_url = %q (err=%v)", out, isErr)
	}
	if out, isErr = call(t, cs, "apk_push_file", map[string]any{"apk": "NewPipe", "file": "../secrets.vault"}); !isErr || fa.ctl.pushed != "" {
		t.Fatalf("apk_push_file allowed traversal: %q", out)
	}
	if out, isErr = call(t, cs, "apk_push_file", map[string]any{"apk": "NewPipe", "file": "clip.mp4"}); isErr || !strings.HasSuffix(fa.ctl.pushed, "clip.mp4") {
		t.Fatalf("apk_push_file = %q (err=%v)", out, isErr)
	}
	if out, isErr = call(t, cs, "apk_pull_file", map[string]any{"apk": "NewPipe", "path": "/sdcard/DCIM/AnyProfile/a.jpg"}); isErr || !strings.Contains(out, "Saved /tmp/file") {
		t.Fatalf("apk_pull_file = %q (err=%v)", out, isErr)
	}
	if out, isErr = call(t, cs, "apk_clipboard_get", map[string]any{"apk": "NewPipe"}); isErr || !strings.Contains(out, "hello") {
		t.Fatalf("apk_clipboard_get = %q (err=%v)", out, isErr)
	}
	if out, isErr = call(t, cs, "apk_clipboard_set", map[string]any{"apk": "NewPipe", "text": "hi"}); isErr {
		t.Fatalf("apk_clipboard_set = %q (err=%v)", out, isErr)
	}
	if out, isErr = call(t, cs, "apk_notifications", map[string]any{"apk": "NewPipe"}); isErr || !strings.Contains(out, "com.test") {
		t.Fatalf("apk_notifications = %q (err=%v)", out, isErr)
	}
	if out, isErr = call(t, cs, "apk_grant_permission", map[string]any{"apk": "NewPipe", "permission": "notifications"}); isErr || !strings.Contains(out, "org.schabi.newpipe") {
		t.Fatalf("apk_grant_permission = %q (err=%v)", out, isErr)
	}
	if out, isErr = call(t, cs, "apk_revoke_permission", map[string]any{"apk": "NewPipe", "permission": "notifications"}); isErr {
		t.Fatalf("apk_revoke_permission = %q (err=%v)", out, isErr)
	}

	// Second MCP session cannot hijack the shared APK runner while held.
	two, err := connect(t, srv, rewrite{token: token})
	if err != nil {
		t.Fatal(err)
	}
	defer two.Close()
	if out, isErr = call(t, two, "apk_tap", map[string]any{"apk": "F-Droid", "ref": "m1"}); !isErr || !strings.Contains(out, "apk_busy") {
		t.Fatalf("foreign session apk_tap = %q (err=%v), want apk_busy", out, isErr)
	}

	// Close releases lease and stops the app.
	if out, isErr = call(t, cs, "close_apk", map[string]any{"apk": "NewPipe"}); isErr || !slices.Equal(fa.closed, []string{"org.schabi.newpipe"}) {
		t.Fatalf("close_apk = %q (err=%v)", out, isErr)
	}
	if strings.Contains(logBuf.String(), "Bí mật") {
		t.Fatal("typed APK text leaked into log")
	}
	if !strings.Contains(logBuf.String(), "tool=apk_type apk=org.schabi.newpipe") {
		t.Fatal("APK tools not logged with package id")
	}
}

func TestMCPAPKManageTools(t *testing.T) {
	fa := &fakeAPK{
		ctl: &fakeControl{},
		apps: []APKApp{
			{Package: "org.schabi.newpipe", Label: "NewPipe", Model: "Pixel 9", AndroidID: "be473c3a44355233"},
		},
	}
	s, srv := apkSetup(t, fa, true)
	os.WriteFile(filepath.Join(s.Uploads, "demo.apk"), []byte("apk"), 0o600)
	os.WriteFile(filepath.Join(s.Uploads, "notes.txt"), []byte("txt"), 0o600)

	cs, err := connect(t, srv, rewrite{token: token})
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	list, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]*mcp.Tool{}
	for _, tl := range list.Tools {
		by[tl.Name] = tl
	}
	for _, name := range []string{"apk_add", "apk_update", "apk_delete"} {
		if by[name] == nil {
			t.Fatalf("%s missing with AgentManage=true", name)
		}
	}
	if a := by["apk_delete"].Annotations; a == nil || a.DestructiveHint == nil || !*a.DestructiveHint {
		t.Fatalf("apk_delete must be marked destructive: %+v", a)
	}

	// Rejects non-APK extension and path traversal.
	if out, isErr := call(t, cs, "apk_add", map[string]any{"file": "notes.txt"}); !isErr || !strings.Contains(out, "mobile_apk") {
		t.Fatalf("apk_add non-apk = %q (err=%v)", out, isErr)
	}
	if out, isErr := call(t, cs, "apk_add", map[string]any{"file": "../evil.apk"}); !isErr {
		t.Fatalf("apk_add traversal accepted: %q", out)
	}
	if out, isErr := call(t, cs, "apk_add", map[string]any{"file": "demo.apk", "label": "Demo App", "proxyId": "px1", "identityTemplate": "pixel9"}); isErr || !strings.Contains(out, `com.demo.app "Demo App"`) {
		t.Fatalf("apk_add = %q (err=%v)", out, isErr)
	}
	if out, isErr := call(t, cs, "apk_update", map[string]any{"apk": "NewPipe", "label": "MyTube", "proxyId": "px2", "identityTemplate": "regen"}); isErr || !strings.Contains(out, `MyTube`) || !strings.Contains(out, "fedcba9876543210") {
		t.Fatalf("apk_update = %q (err=%v)", out, isErr)
	}
	// Delete requires confirm: true.
	if out, isErr := call(t, cs, "apk_delete", map[string]any{"apk": "MyTube", "confirm": false}); !isErr || !strings.Contains(out, "confirm_required") {
		t.Fatalf("apk_delete confirm=false = %q (err=%v)", out, isErr)
	}
	if out, isErr := call(t, cs, "apk_delete", map[string]any{"apk": "MyTube", "confirm": true, "uninstall": true}); isErr || !slices.Equal(fa.deleted, []string{"org.schabi.newpipe"}) {
		t.Fatalf("apk_delete = %q (err=%v), deleted=%v", out, isErr, fa.deleted)
	}
}

func TestMCPNoAPKToolsWhenNil(t *testing.T) {
	_, _, srv := mcpSetup(t)
	cs, err := connect(t, srv, rewrite{token: token})
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	for _, name := range toolNames(t, cs) {
		if strings.Contains(name, "apk") {
			t.Fatalf("%s registered when Server.APK is nil", name)
		}
	}
}
