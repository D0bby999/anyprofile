// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package launcher

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/agent"
	"github.com/D0bby999/anyprofile/internal/engine/apostate"
	"github.com/D0bby999/anyprofile/internal/store"
)

// What sites do that used to stop an agent: a browser dialog (alert,
// confirm, "Leave site?"), a native <select>, and content in iframes, of the
// same origin and of another one (sign-in boxes, captchas).
const casesPage = `<!doctype html><title>Cases</title>
<p id="out">nothing</p>
<button onclick="alert('Saved!'); out('after alert')">Alert me</button>
<button onclick="out(confirm('Delete it?') ? 'confirmed' : 'kept')">Delete</button>
<label>Fruit <select id="fruit" onchange="out('chose ' + this.value)">
  <option value="">Pick one</option><option value="apple">Apple</option><option value="banana">Banana</option>
</select></label>
<label>Draft <input id="draft" oninput="window.onbeforeunload = e => { e.preventDefault(); e.returnValue = '' }"></label>
<iframe src="/inner" title="Same origin"></iframe>
<iframe src="OTHER/inner" title="Other origin"></iframe>
<script>function out(s) { document.getElementById('out').textContent = s }</script>`

const innerPage = `<!doctype html><title>Inner</title>
<button onclick="this.textContent = 'Inner clicked'">Inner button</button>
<label>Inner field <input aria-label="Inner field"></label>`

func TestAgentHandlesDialogsSelectsAndFrames(t *testing.T) {
	r := newApostateRig(t)
	l := r.launcher(t)
	l.Capture = true
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { w.Write([]byte(innerPage)) }))
	defer other.Close()
	// Another site: localhost instead of 127.0.0.1, so a separate process (OOPIF).
	_, port, _ := net.SplitHostPort(other.Listener.Addr().String())
	otherURL := "http://localhost:" + port
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/inner":
			w.Write([]byte(innerPage))
		case "/next":
			w.Write([]byte(`<!doctype html><title>Next</title><p>next page</p>`))
		default:
			w.Write([]byte(strings.Replace(casesPage, "OTHER", otherURL, 1)))
		}
	}))
	defer srv.Close()
	const id = "cases"
	if err := r.st.CreateProfile(&store.Profile{ID: id, Name: "Cases", Engine: "apostate",
		EngineVersion: apostate.Version, PersonaID: apostate.HostPersona, Platform: apostate.HostPlatform,
		SeedHex: "00000000000ca5e5", Timezone: "UTC", Languages: "en-US,en", UILocale: "en-US",
		StartURL: srv.URL + "/"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := l.StartBehind(ctx, id); err != nil {
		t.Fatal(err)
	}
	defer l.Stop(id)
	c, err := l.WaitConn(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	hub := agent.NewHub(time.Minute)
	defer hub.Close()
	b := hub.For(id, c)
	if _, err := b.WaitFor(ctx, "", "nothing", false, "", 20*time.Second); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second) // the iframes load
	snap := func() string {
		t.Helper()
		s, err := b.Snapshot(ctx, "", "")
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	ref := func(s, role, name string) string {
		t.Helper()
		for _, line := range strings.Split(s, "\n") {
			if strings.Contains(line, "- "+role+` "`+name+`"`) {
				if i := strings.Index(line, "[ref="); i >= 0 {
					return line[i+5 : i+5+strings.Index(line[i+5:], "]")]
				}
			}
		}
		t.Fatalf("no %s %q in:\n%s", role, name, s)
		return ""
	}
	out := func() string {
		t.Helper()
		txt, err := b.ReadPage(ctx, "", "", 0, false)
		if err != nil {
			t.Fatal(err)
		}
		return txt
	}
	// within fails the test when a call takes longer than d: a dialog that
	// blocks the page used to hang every call after it.
	within := func(what string, d time.Duration, fn func() error) {
		t.Helper()
		done := make(chan error, 1)
		go func() { done <- fn() }()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("%s: %v", what, err)
			}
		case <-time.After(d):
			t.Fatalf("%s: still waiting after %v", what, d)
		}
	}

	s := snap()
	t.Logf("snapshot:\n%s", s)

	t.Run("alert", func(t *testing.T) {
		var tab agent.Tab
		within("click Alert me", 15*time.Second, func() (err error) {
			tab, err = b.Click(ctx, "", ref(s, "button", "Alert me"))
			return err
		})
		if !strings.Contains(tab.Dialog, "Saved!") {
			t.Errorf("the alert is not reported: %+v", tab)
		}
		if txt := out(); !strings.Contains(txt, "after alert") {
			t.Errorf("page after the alert: %q", txt)
		}
	})
	t.Run("confirm", func(t *testing.T) {
		within("click Delete", 15*time.Second, func() error { _, err := b.Click(ctx, "", ref(s, "button", "Delete")); return err })
		if txt := out(); !strings.Contains(txt, "confirmed") {
			t.Errorf("a confirm after the agent's own click should be accepted: %q", txt)
		}
	})
	t.Run("select", func(t *testing.T) {
		within("select Banana", 15*time.Second, func() error {
			_, err := b.SelectOption(ctx, "", ref(s, "combobox", "Fruit"), "Banana")
			return err
		})
		if txt := out(); !strings.Contains(txt, "chose banana") {
			t.Errorf("page after selecting: %q", txt)
		}
		if _, err := b.SelectOption(ctx, "", ref(s, "combobox", "Fruit"), "Cherry"); err == nil || !strings.Contains(err.Error(), "Banana") {
			t.Errorf("an unknown option should list the choices: %v", err)
		}
	})
	t.Run("frames", func(t *testing.T) {
		buttons := 0
		for _, line := range strings.Split(s, "\n") {
			if strings.Contains(line, `button "Inner button"`) {
				buttons++
			}
		}
		if buttons != 2 {
			t.Fatalf("want the button of both iframes in the snapshot, got %d", buttons)
		}
		for _, line := range strings.Split(s, "\n") {
			if !strings.Contains(line, `button "Inner button"`) {
				continue
			}
			i := strings.Index(line, "[ref=")
			r := line[i+5 : i+5+strings.Index(line[i+5:], "]")]
			within("click "+r, 15*time.Second, func() error { _, err := b.Click(ctx, "", r); return err })
		}
		if got := strings.Count(snap(), `button "Inner clicked"`); got != 2 {
			t.Errorf("clicked %d of the 2 iframe buttons:\n%s", got, snap())
		}
		// Typing reaches the field it was asked to, in either iframe.
		for i, line := range strings.Split(snap(), "\n") {
			if !strings.Contains(line, `textbox "Inner field"`) {
				continue
			}
			j := strings.Index(line, "[ref=")
			r := line[j+5 : j+5+strings.Index(line[j+5:], "]")]
			within("type in "+r, 15*time.Second, func() error { _, err := b.Type(ctx, "", r, fmt.Sprintf("hello%d", i), false, false); return err })
		}
		after := snap()
		if got := strings.Count(after, `[value="hello`); got != 2 {
			t.Errorf("typed into %d of the 2 iframe fields:\n%s", got, after)
		}
	})
	t.Run("leave site", func(t *testing.T) {
		within("type a draft", 15*time.Second, func() error {
			_, err := b.Type(ctx, "", ref(s, "textbox", "Draft"), "unsaved", false, false)
			return err
		})
		var tab agent.Tab
		within("navigate away", 20*time.Second, func() (err error) { tab, err = b.Navigate(ctx, "", srv.URL+"/next", ""); return err })
		if !strings.Contains(tab.Dialog, "leave") || strings.Contains(tab.URL, "/next") {
			t.Fatalf("the first try should stay and say why: %+v", tab)
		}
		within("navigate again", 20*time.Second, func() (err error) { tab, err = b.Navigate(ctx, "", srv.URL+"/next", ""); return err })
		if !strings.Contains(tab.URL, "/next") {
			t.Fatalf("asked twice, the agent should leave: %+v", tab)
		}
	})
}
