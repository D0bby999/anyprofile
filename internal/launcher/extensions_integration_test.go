// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package launcher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/bookmarks"
	"github.com/D0bby999/anyprofile/internal/engine/apostate"
	"github.com/D0bby999/anyprofile/internal/engine/fury"
	"github.com/D0bby999/anyprofile/internal/store"
)

// An extension assigned to a profile runs in it; one not assigned does not.
func TestExtensionLoads(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("<title>ok</title>")) }))
	defer srv.Close()
	for _, name := range []string{"apostate", "fury"} {
		t.Run(name, func(t *testing.T) {
			var r *rig
			p := &store.Profile{ID: "ext-" + name, Name: "Ext", SeedHex: "00000000000e0e0e", Timezone: "UTC",
				Languages: "en-US,en", UILocale: "en-US", StartURL: srv.URL + "/"}
			if name == "apostate" {
				r = newApostateRig(t)
				p.Engine, p.EngineVersion, p.PersonaID, p.Platform = "apostate", apostate.Version, apostate.HostPersona, apostate.HostPlatform
			} else {
				r = newRig(t)
				ident, err := r.eng.NewIdentity(context.Background(), 7)
				if err != nil {
					t.Fatal(err)
				}
				p.Engine, p.EngineVersion, p.PersonaID, p.Platform = "fury", fury.Version, ident.PersonaID, ident.Platform
			}
			dir := r.dirs.Extension("ext1")
			os.MkdirAll(dir, 0o700)
			os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"manifest_version": 3, "name": "Mark", "version": "1",
				"content_scripts": [{"matches": ["<all_urls>"], "js": ["cs.js"], "run_at": "document_start"}]}`), 0o600)
			os.WriteFile(filepath.Join(dir, "cs.js"), []byte(`document.documentElement.dataset.apMark = "1";`), 0o600)
			if err := r.st.CreateExtension(&store.Extension{ID: "ext1", Name: "Mark", Version: "1"}); err != nil {
				t.Fatal(err)
			}
			if err := r.st.CreateProfile(p); err != nil {
				t.Fatal(err)
			}
			l := r.launcher(t)
			l.Capture = true
			if got := marked(t, l, p.ID, srv.URL+"/a"); got != "none" {
				t.Fatalf("not assigned, but the content script ran: %q", got)
			}
			r.st.SetProfileExtension(p.ID, "ext1", true)
			if got := marked(t, l, p.ID, srv.URL+"/b"); got != "1" {
				t.Fatalf("assigned, content script = %q", got)
			}
		})
	}
}

// marked opens the profile, loads the test page in a new tab, reads the mark
// the test extension sets there, and closes the profile. A new tab, not the
// restored one: a tab Chromium restores at startup can load before an
// unpacked extension is ready (see restoredTabMark).
func marked(t *testing.T, l *Launcher, id, url string) string {
	t.Helper()
	ctx := context.Background()
	if err := l.StartBehind(ctx, id); err != nil {
		t.Fatal(err)
	}
	defer l.Stop(id)
	t.Cleanup(func() {
		if t.Failed() {
			b, _ := os.ReadFile(filepath.Join(l.dirs.Logs(), "browser-"+id+".log"))
			t.Logf("browser log:\n%s", b)
		}
	})
	wctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	c, err := l.WaitConn(wctx, id)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	raw, err := c.Call(ctx, "", "Target.createTarget", map[string]any{"url": url})
	if err != nil {
		t.Fatal(err)
	}
	var tg struct{ TargetID string }
	json.Unmarshal(raw, &tg)
	return markIn(t, c, tg.TargetID)
}

func markIn(t *testing.T, c interface {
	Call(context.Context, string, string, any) (json.RawMessage, error)
}, targetID string) string {
	t.Helper()
	ctx := context.Background()
	raw, err := c.Call(ctx, "", "Target.attachToTarget", map[string]any{"targetId": targetID, "flatten": true})
	if err != nil {
		t.Fatal(err)
	}
	var att struct{ SessionID string }
	json.Unmarshal(raw, &att)
	// A content script runs at document_idle, which a slow machine reaches
	// after readyState is complete: "none" is only final after a grace.
	var noneSince time.Time
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(300 * time.Millisecond) {
		raw, err = c.Call(ctx, att.SessionID, "Runtime.evaluate", map[string]any{
			"expression": `document.readyState === "complete" ? (document.documentElement.dataset.apMark || "none") : ""`, "returnByValue": true})
		if err != nil {
			continue
		}
		var v struct{ Result struct{ Value string } }
		json.Unmarshal(raw, &v)
		switch {
		case v.Result.Value == "none" && noneSince.IsZero():
			noneSince = time.Now()
		case v.Result.Value == "none" && time.Since(noneSince) > 3*time.Second:
			return "none"
		case v.Result.Value != "" && v.Result.Value != "none":
			return v.Result.Value
		}
	}
	if !noneSince.IsZero() {
		return "none"
	}
	t.Fatal("page never loaded")
	return ""
}

// Chromium reads the Bookmarks file the importer writes.
func TestImportedBookmarksShowInChromium(t *testing.T) {
	r := newApostateRig(t)
	l := r.launcher(t)
	l.Capture = true
	p := &store.Profile{ID: "bm1", Name: "Bm", Engine: "apostate", EngineVersion: apostate.Version, PersonaID: apostate.HostPersona,
		Platform: apostate.HostPlatform, SeedHex: "00000000000f0f0f", Timezone: "UTC", Languages: "en-US,en", UILocale: "en-US"}
	if err := r.st.CreateProfile(p); err != nil {
		t.Fatal(err)
	}
	root, _, err := bookmarks.Parse([]byte(`<DL><p><DT><H3>Shop</H3><DL><p><DT><A HREF="https://example.com/shop">My shop</A></DL><p></DL>`))
	if err != nil {
		t.Fatal(err)
	}
	if err := bookmarks.AddToProfile(r.dirs.Profile(p.ID), "Imported", root); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := l.StartBehind(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	defer l.Stop(p.ID)
	wctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	c, err := l.WaitConn(wctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := c.Call(ctx, "", "Target.createTarget", map[string]any{"url": "chrome://bookmarks/"})
	if err != nil {
		t.Fatal(err)
	}
	var tg struct{ TargetID string }
	json.Unmarshal(raw, &tg)
	raw, _ = c.Call(ctx, "", "Target.attachToTarget", map[string]any{"targetId": tg.TargetID, "flatten": true})
	var att struct{ SessionID string }
	json.Unmarshal(raw, &att)
	var got string
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline) && got == ""; time.Sleep(300 * time.Millisecond) {
		raw, err = c.Call(ctx, att.SessionID, "Runtime.evaluate", map[string]any{"awaitPromise": true, "returnByValue": true,
			"expression": `new Promise((ok) => chrome.bookmarks.search({url: "https://example.com/shop"}, (r) => ok(r.length ? r[0].title : "")))`})
		if err == nil {
			var v struct{ Result struct{ Value string } }
			json.Unmarshal(raw, &v)
			got = v.Result.Value
		}
	}
	if got != "My shop" {
		t.Fatalf("bookmark title in Chromium = %q", got)
	}
}
