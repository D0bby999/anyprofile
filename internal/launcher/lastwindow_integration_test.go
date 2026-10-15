// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package launcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/engine/apostate"
	"github.com/D0bby999/anyprofile/internal/engine/fury"
	"github.com/D0bby999/anyprofile/internal/store"
)

// Closing the last window (every page gone) quits the browser by itself,
// while a page closed and another opened at once keeps it running.
func TestLastWindowClosedQuits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("<title>ok</title>")) }))
	defer srv.Close()
	for _, name := range []string{"apostate", "fury"} {
		t.Run(name, func(t *testing.T) {
			var r *rig
			p := &store.Profile{ID: "lw-" + name, Name: "Last window", SeedHex: "00000000000d0d0d", Timezone: "UTC",
				Languages: "en-US,en", UILocale: "en-US", StartURL: srv.URL + "/"}
			if name == "apostate" {
				r = newApostateRig(t)
				p.Engine, p.EngineVersion, p.PersonaID, p.Platform = "apostate", apostate.Version, "apostate-macos", "macos"
			} else {
				r = newRig(t)
				ident, err := r.eng.NewIdentity(context.Background(), 7)
				if err != nil {
					t.Fatal(err)
				}
				p.Engine, p.EngineVersion, p.PersonaID, p.Platform = "fury", fury.Version, ident.PersonaID, ident.Platform
			}
			if err := r.st.CreateProfile(p); err != nil {
				t.Fatal(err)
			}
			l := r.launcher(t)
			l.Capture = true // as in the app: the pipe is how the launcher sees the pages
			ctx := context.Background()
			if err := l.Start(ctx, p.ID); err != nil {
				t.Fatal(err)
			}
			defer l.Stop(p.ID)
			wctx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			c, err := l.WaitConn(wctx, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			time.Sleep(2 * windowsEvery)

			// Swap the page for another: the browser must stay.
			ts, _ := pageTargets(ctx, c)
			if _, err := c.Call(ctx, "", "Target.createTarget", map[string]any{"url": srv.URL + "/next"}); err != nil {
				t.Fatal(err)
			}
			for _, tg := range ts {
				c.Call(ctx, "", "Target.closeTarget", map[string]any{"targetId": tg.TargetID})
			}
			time.Sleep((emptyChecks + 2) * windowsEvery)
			if in, ok := l.Info(p.ID); !ok || in.State != Running {
				t.Fatalf("browser quit while a page was open: %+v %v", in, ok)
			}

			// Close every page, as the window's close button does.
			ts, _ = pageTargets(ctx, c)
			for _, tg := range ts {
				c.Call(ctx, "", "Target.closeTarget", map[string]any{"targetId": tg.TargetID})
			}
			waitStopped(t, l, p.ID)
		})
	}
}
