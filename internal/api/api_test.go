// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/D0bby999/anyprofile/internal/agent"
	"github.com/D0bby999/anyprofile/internal/apperr"
)

type fake struct {
	profiles []Profile
	started  []string
	devtools bool
	stopped  []string
	startErr error
	pipe     agent.Conn
	adopted  bool // running browsers have no pipe
}

func (f *fake) Profiles() ([]Profile, error) { return f.profiles, nil }
func (f *fake) Start(_ context.Context, id string, devtools bool) error {
	if f.startErr != nil {
		return f.startErr
	}
	f.started, f.devtools = append(f.started, id), devtools
	for i := range f.profiles {
		if f.profiles[i].ID == id {
			f.profiles[i].State = "running"
			if devtools {
				f.profiles[i].DevToolsPort = 9333
			}
		}
	}
	return nil
}
func (f *fake) Stop(id string) error { f.stopped = append(f.stopped, id); return nil }

const port, token = 51525, "s3cret-token"

func setup() (*fake, http.Handler) {
	f := &fake{profiles: []Profile{{ID: "p1", Serial: 1, Name: "Kênh", Sites: []string{"tiktok"}, Tags: []string{}, State: "stopped"}}}
	s := New(f, slog.New(slog.NewTextHandler(io.Discard, nil)), port, token)
	s.devtoolsURL = func(p int) string { return "ws://127.0.0.1:9333/devtools/browser/x" }
	return f, s.Handler()
}

func do(h http.Handler, method, path, body string, edit func(*http.Request)) (int, map[string]any) {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Host = "127.0.0.1:51525"
	r.Header.Set("Authorization", "Bearer "+token)
	if edit != nil {
		edit(r)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func errCode(out map[string]any) string {
	e, _ := out["error"].(map[string]any)
	s, _ := e["code"].(string)
	return s
}

func TestRefusesWhatAWebPageOrAStrangerCouldSend(t *testing.T) {
	f, h := setup()
	cases := []struct {
		name string
		edit func(*http.Request)
		code int
		want string
	}{
		{"no token", func(r *http.Request) { r.Header.Del("Authorization") }, 401, "api_token"},
		{"wrong token", func(r *http.Request) { r.Header.Set("Authorization", "Bearer nope") }, 401, "api_token"},
		{"token without Bearer", func(r *http.Request) { r.Header.Set("Authorization", token) }, 401, "api_token"},
		{"a web page", func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") }, 403, "api_origin"},
		{"DNS rebinding", func(r *http.Request) { r.Host = "evil.example:51525" }, 403, "api_host"},
		{"another port", func(r *http.Request) { r.Host = "127.0.0.1:9999" }, 403, "api_host"},
	}
	for _, c := range cases {
		code, out := do(h, "POST", "/v1/profiles/p1/start", "", c.edit)
		if code != c.code || errCode(out) != c.want {
			t.Errorf("%s: %d %v, want %d %s", c.name, code, out, c.code, c.want)
		}
	}
	if len(f.started) != 0 {
		t.Fatalf("a refused request started %v", f.started)
	}
	if code, _ := do(h, "GET", "/v1/profiles", "", func(r *http.Request) { r.Host = "localhost:51525" }); code != 200 {
		t.Fatalf("localhost host = %d", code)
	}
}

func TestStartWithDevToolsThenStop(t *testing.T) {
	f, h := setup()
	code, out := do(h, "POST", "/v1/profiles/p1/start", `{"devtools": true}`, nil)
	if code != 200 || out["state"] != "running" || out["devtoolsPort"].(float64) != 9333 || out["webSocketDebuggerUrl"] == "" {
		t.Fatalf("start = %d %v", code, out)
	}
	if !f.devtools || len(f.started) != 1 {
		t.Fatalf("backend start = %v devtools=%v", f.started, f.devtools)
	}
	if code, _ := do(h, "POST", "/v1/profiles/p1/stop", "", nil); code != 200 || len(f.stopped) != 1 {
		t.Fatalf("stop = %d %v", code, f.stopped)
	}
	code, out = do(h, "GET", "/v1/profiles", "", nil)
	if ps, _ := out["profiles"].([]any); code != 200 || len(ps) != 1 {
		t.Fatalf("list = %d %v", code, out)
	}
}

func TestErrors(t *testing.T) {
	f, h := setup()
	if code, out := do(h, "GET", "/v1/profiles/nope", "", nil); code != 404 || errCode(out) != "not_found" {
		t.Fatalf("unknown profile = %d %v", code, out)
	}
	if code, out := do(h, "POST", "/v1/profiles/p1/start", `{devtools`, nil); code != 400 || errCode(out) != "api_bad_body" {
		t.Fatalf("bad body = %d %v", code, out)
	}
	f.startErr = apperr.New("profile_trashed", "in the trash")
	if code, out := do(h, "POST", "/v1/profiles/p1/start", "", nil); code != 409 || errCode(out) != "profile_trashed" {
		t.Fatalf("app error = %d %v", code, out)
	}
	if code, _ := do(h, "DELETE", "/v1/profiles/p1", "", nil); code != 405 {
		t.Fatalf("DELETE = %d, want 405", code)
	}
}
