// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package launcher

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/agent"
	"github.com/D0bby999/anyprofile/internal/engine/apostate"
	"github.com/D0bby999/anyprofile/internal/store"
)

// The page reports, in its own DOM, what a bot check would see: whether a
// DevTools client enabled Runtime (the stack getter fires when console
// messages are serialised for one), whether input events were trusted, and
// whether the viewport changed.
// Two upload forms: a labelled file input, and YouTube's kind, a button that
// clicks a hidden input. Each writes what it got into the page.
const uploadPage = `<!doctype html><title>Upload test</title>
<label>Video file <input type="file" id="plain" onchange="got('plain', this)"></label>
<input type="file" id="hidden" multiple style="display:none" onchange="got('hidden', this)">
<button onclick="document.getElementById('hidden').click()">Select files</button>
<p id="out">nothing yet</p>
<script>function got(which, el) { document.getElementById('out').textContent += ' | ' + which + ': ' + [...el.files].map(f => f.name + '/' + f.size).join(','); }</script>`

const agentPage = `<!doctype html><title>Agent test</title>
<h1>Agent test</h1>
<p id="cdp">CDP-CLEAN</p><p id="resize">RESIZE-0</p><p id="clicked">not clicked</p><p id="keys">no keys</p>
<label>Search <input id="q" aria-label="Search"></label>
<button id="go" onclick="go(event)">Go</button>
<div style="height:3000px"></div><button id="far" onclick="this.textContent='far clicked'">Far</button>
<script>
const e = new Error();
Object.defineProperty(e, 'stack', { get() { document.getElementById('cdp').textContent = 'CDP-SEEN'; return ''; } });
setInterval(() => console.debug(e), 100);
let n = 0; addEventListener('resize', () => document.getElementById('resize').textContent = 'RESIZE-' + (++n));
document.getElementById('q').addEventListener('keydown', ev => { if (ev.isTrusted) document.getElementById('keys').textContent = 'trusted keys'; });
async function go(ev) {
  const r = await fetch('/api/data?q=' + encodeURIComponent(document.getElementById('q').value));
  const j = await r.json();
  document.getElementById('clicked').textContent = (ev.isTrusted ? 'trusted click: ' : 'untrusted click: ') + j.echo;
}
</script>`

func TestAgentDrivesApostateOverThePipe(t *testing.T) {
	r := newApostateRig(t)
	l := r.launcher(t)
	l.Capture = true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/upload" {
			w.Write([]byte(uploadPage))
			return
		}
		if req.URL.Path == "/api/data" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"echo":%q}`, req.URL.Query().Get("q"))
			return
		}
		w.Write([]byte(agentPage))
	}))
	defer srv.Close()
	const id = "ag1"
	if err := r.st.CreateProfile(&store.Profile{ID: id, Name: "Agent", Engine: "apostate",
		EngineVersion: apostate.Version, PersonaID: apostate.HostPersona, Platform: apostate.HostPlatform,
		SeedHex: "00000000000a9e47", Timezone: "UTC", Languages: "en-US,en", UILocale: "en-US",
		StartURL: srv.URL + "/"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := l.Start(ctx, id); err != nil {
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

	tab, err := b.WaitFor(ctx, "", "Agent test", false, "", 20*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tab.URL, srv.URL) {
		t.Fatalf("current tab = %+v", tab)
	}
	// The window settles at launch (a resize or two); the agent must add none.
	time.Sleep(time.Second)
	before, _ := b.ReadPage(ctx, "", "", 0, false)
	resized := regexp.MustCompile(`RESIZE-\d+`).FindString(before)
	snap, err := b.Snapshot(ctx, "", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("snapshot:\n%s", snap)
	ref := func(role, name string) string {
		for _, line := range strings.Split(snap, "\n") {
			if strings.Contains(line, "- "+role+` "`+name+`"`) {
				if i := strings.Index(line, "[ref="); i >= 0 {
					return line[i+5 : i+5+strings.Index(line[i+5:], "]")]
				}
			}
		}
		t.Fatalf("no %s %q in snapshot", role, name)
		return ""
	}
	if _, err := b.Type(ctx, "", ref("textbox", "Search"), "xin chào 42", true, false); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Click(ctx, "", ref("button", "Go")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.WaitFor(ctx, "", "trusted click: xin chào 42", false, "", 10*time.Second); err != nil {
		text, _ := b.ReadPage(ctx, "", "", 0, false)
		t.Fatalf("%v; page:\n%s", err, text)
	}
	// The element below the fold: scrolled into view and clicked there.
	if _, err := b.Click(ctx, "", ref("button", "Far")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.WaitFor(ctx, "", "far clicked", false, "", 5*time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Scroll(ctx, "", "", -3); err != nil {
		t.Fatal(err)
	}

	reqs, err := b.Requests(ctx, "", "/api/data", nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	reqID := strings.Fields(strings.Split(reqs, "\n")[1])[0]
	body, err := b.ResponseBody(ctx, "", reqID, 0)
	if err != nil || !strings.Contains(body, `{"echo":"xin chào 42"}`) {
		t.Fatalf("body = %q, %v (requests:\n%s)", body, err, reqs)
	}

	shot, err := b.Screenshot(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.DecodeConfig(bytes.NewReader(shot))
	if err != nil || img.Width > agent.MaxShotWidth || img.Width < 200 {
		t.Fatalf("screenshot %dx%d, %v", img.Width, img.Height, err)
	}

	page, err := b.ReadPage(ctx, "", "", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"CDP-CLEAN", resized, "trusted keys"} {
		if !strings.Contains(page, want) {
			t.Errorf("page should show %q:\n%s", want, page)
		}
	}

	// Uploads: straight into a file input, and through a file chooser.
	uploads := t.TempDir()
	os.WriteFile(filepath.Join(uploads, "clip.mp4"), make([]byte, 1234), 0o600)
	os.WriteFile(filepath.Join(uploads, "thumb.jpg"), make([]byte, 56), 0o600)
	if _, err := b.Navigate(ctx, "", srv.URL+"/upload", ""); err != nil {
		t.Fatal(err)
	}
	usnap, err := b.Snapshot(ctx, "", "")
	if err != nil {
		t.Fatal(err)
	}
	snap = usnap
	t.Logf("upload snapshot:\n%s", usnap)
	one, err := agent.ResolveUploads(uploads, []string{"clip.mp4"})
	if err != nil {
		t.Fatal(err)
	}
	both, _ := agent.ResolveUploads(uploads, []string{"clip.mp4", "thumb.jpg"})
	if _, err := b.UploadFile(ctx, "", ref("button", "Video file"), one); err != nil {
		t.Fatal(err)
	}
	if _, err := b.UploadFile(ctx, "", ref("button", "Select files"), both); err != nil {
		t.Fatal(err)
	}
	if _, err := b.WaitFor(ctx, "", "hidden: clip.mp4/1234,thumb.jpg/56", false, "", 5*time.Second); err != nil {
		text, _ := b.ReadPage(ctx, "", "", 0, false)
		t.Fatalf("%v; page:\n%s", err, text)
	}
	if text, _ := b.ReadPage(ctx, "", "", 0, false); !strings.Contains(text, "plain: clip.mp4/1234") {
		t.Fatalf("plain input:\n%s", text)
	}

	opened, err := b.OpenTab(ctx, srv.URL+"/second")
	if err != nil {
		t.Fatal(err)
	}
	// It answers once the page is there, and its own request is in the log.
	if !strings.HasSuffix(opened.URL, "/second") {
		t.Fatalf("open_tab answered before the page: %+v", opened)
	}
	if reqs, err := b.Requests(ctx, opened.ID, "/second", []string{"Document"}, 0); err != nil || !strings.Contains(reqs, " 200 ") {
		t.Fatalf("the new tab's own request is not logged: %q, %v", reqs, err)
	}
	tabs, _ := b.Tabs(ctx)
	if len(tabs) != 2 || !tabs[0].Current && !tabs[1].Current {
		t.Fatalf("tabs = %+v", tabs)
	}
	if err := b.CloseTab(ctx, opened.ID); err != nil {
		t.Fatal(err)
	}
	if got := hub.Active(); len(got) != 1 || got[0] != id {
		t.Fatalf("active = %v", got)
	}
}
