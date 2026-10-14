// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package launcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/cdppipe"
	"github.com/D0bby999/anyprofile/internal/engine/apostate"
	"github.com/D0bby999/anyprofile/internal/store"
)

// The bug this guards: closing the window before Stop left Chromium with an
// empty session, so the next launch showed only a new tab.
func TestApostateReopensTabsAfterWindowClosed(t *testing.T) {
	r := newApostateRig(t)
	l := r.launcher(t)
	l.Capture = true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("<title>ok</title>")) }))
	defer srv.Close()
	const id = "ap3"
	if err := r.st.CreateProfile(&store.Profile{ID: id, Name: "Tabs", Engine: "apostate",
		EngineVersion: apostate.Version, PersonaID: "apostate-macos", Platform: "macos",
		SeedHex: "00000000000c0c0c", Timezone: "UTC", Languages: "en-US,en", UILocale: "en-US",
		StartURL: srv.URL + "/start"}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	launch := func() *cdppipe.Conn {
		t.Helper()
		if err := l.Start(ctx, id); err != nil {
			t.Fatal(err)
		}
		wctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		c, err := l.WaitConn(wctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	// waitTabs polls until the web tabs, sorted, equal want.
	waitTabs := func(c *cdppipe.Conn, want ...string) {
		t.Helper()
		var got []string
		for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(300 * time.Millisecond) {
			ts, err := pageTargets(ctx, c)
			if err != nil {
				continue
			}
			got = webTabs(ts)
			slices.Sort(got)
			if slices.Equal(got, want) {
				return
			}
		}
		t.Fatalf("tabs = %v, want %v", got, want)
	}

	// Launch 1: the start page, plus a tab the user opens.
	c := launch()
	waitTabs(c, srv.URL+"/start")
	if _, err := c.Call(ctx, "", "Target.createTarget", map[string]any{"url": srv.URL + "/two"}); err != nil {
		t.Fatal(err)
	}
	waitTabs(c, srv.URL+"/start", srv.URL+"/two")
	time.Sleep(tabsEvery + time.Second) // let the list be saved
	// Close every tab, as closing the window does, then Stop.
	ts, _ := pageTargets(ctx, c)
	for _, tg := range ts {
		if !strings.HasSuffix(tg.URL, ".invalid/") {
			c.Call(ctx, "", "Target.closeTarget", map[string]any{"targetId": tg.TargetID})
		}
	}
	time.Sleep(tabsEvery + time.Second) // an empty list must not replace the saved one
	if err := l.Stop(id); err != nil {
		t.Fatal(err)
	}

	// Launch 2: both tabs come back, and no empty new tab is left.
	c = launch()
	waitTabs(c, srv.URL+"/start", srv.URL+"/two")
	time.Sleep(time.Second)
	ts, _ = pageTargets(ctx, c)
	for _, tg := range ts {
		if isNewTab(tg.URL) {
			t.Fatalf("new tab left open: %v", ts)
		}
	}
	if err := l.Stop(id); err != nil {
		t.Fatal(err)
	}

	// "Open start pages": only the start page, whatever was open before.
	if err := r.st.SetSetting(store.SettingOnOpen, "start"); err != nil {
		t.Fatal(err)
	}
	c = launch()
	waitTabs(c, srv.URL+"/start")
	if err := l.Stop(id); err != nil {
		t.Fatal(err)
	}
}
