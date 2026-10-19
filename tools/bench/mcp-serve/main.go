// SPDX-License-Identifier: AGPL-3.0-or-later

// mcp-serve runs AnyProfile's real launcher and API, /mcp included, on a
// throwaway data folder with one profile, so an AI agent can be tried against
// it without touching the user's profiles. It also serves a small test shop
// on 127.0.0.1:51627. Closing stdin stops it.
// Developer tool, not part of the app.
//
//	ANYPROFILE_TOKEN=… go run ./tools/bench/mcp-serve -app <Chromium.app of Apostate>
//	claude mcp add --transport http anyprofile-bench http://127.0.0.1:51626/mcp \
//	  --header "Authorization: Bearer ${ANYPROFILE_TOKEN}"
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/agent"
	"github.com/D0bby999/anyprofile/internal/api"
	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/engine/apostate"
	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/service"
	"github.com/D0bby999/anyprofile/internal/store"
)

const shopPage = `<!doctype html><title>Test Shop</title>
<h1>Test Shop</h1>
<p id="cdp">bot check: clean</p>
<form onsubmit="event.preventDefault(); search()">
<input id="q" aria-label="Search products"> <button>Search</button>
</form>
<ul id="results"></ul>
<script>
const e = new Error();
Object.defineProperty(e, 'stack', { get() { document.getElementById('cdp').textContent = 'bot check: DevTools seen'; return ''; } });
setInterval(() => console.debug(e), 200);
async function search() {
  const r = await fetch('/api/search?q=' + encodeURIComponent(document.getElementById('q').value));
  const items = await r.json();
  document.getElementById('results').innerHTML = items.map(i => '<li>' + i.name + ' — ' + i.price + ' ₫</li>').join('');
}
</script>`

// uploadPage works like YouTube Studio's: a button opens a hidden file input.
const uploadPage = `<!doctype html><title>Upload video</title>
<h1>Upload video</h1>
<input type="file" id="f" accept="video/*" style="display:none" onchange="document.getElementById('got').textContent = 'Uploaded: ' + [...this.files].map(f => f.name + ' (' + f.size + ' bytes)').join(', ')">
<button onclick="document.getElementById('f').click()">Select files</button>
<p id="got">No file yet</p>`

type backend struct{ svc *service.Service }

func (b backend) Profiles() ([]api.Profile, error) {
	rows, err := b.svc.LiveProfiles()
	out := make([]api.Profile, 0, len(rows))
	for _, r := range rows {
		out = append(out, api.Profile{ID: r.ID, Serial: r.Serial, Name: r.Name, Sites: r.Sites, Tags: r.Tags,
			State: string(r.State), DevToolsPort: r.CDPPort})
	}
	return out, err
}
func (b backend) Start(ctx context.Context, id string, devtools bool) error {
	return b.svc.StartProfile(ctx, id, devtools)
}
func (b backend) Stop(id string) error { return b.svc.Launcher.Stop(id) }
func (b backend) Pipe(id string) (agent.Conn, bool) {
	c, ok := b.svc.Launcher.Conn(id)
	if !ok {
		return nil, false
	}
	return c, true
}

func main() {
	app := flag.String("app", "", "Chromium.app of Apostate")
	port := flag.Int("port", 51626, "API port")
	headless := flag.Bool("headless", false, "run the browsers without a window")
	profiles := flag.Int("profiles", 1, "how many test profiles to make")
	flag.Parse()
	if err := run(*app, *port, *headless, *profiles); err != nil {
		fmt.Fprintln(os.Stderr, "mcp-serve:", err)
		os.Exit(1)
	}
}

func run(app string, port int, headless bool, profiles int) error {
	token := os.Getenv("ANYPROFILE_TOKEN")
	if token == "" {
		return fmt.Errorf("set ANYPROFILE_TOKEN")
	}
	shop := http.NewServeMux()
	shop.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		q := strings.ToLower(r.URL.Query().Get("q"))
		all := []struct{ Name, Price string }{{"Áo thun trắng", "150.000"}, {"Áo khoác gió", "420.000"}, {"Quần jean", "380.000"}}
		w.Header().Set("Content-Type", "application/json")
		var parts []string
		for _, it := range all {
			if strings.Contains(strings.ToLower(it.Name), q) {
				parts = append(parts, fmt.Sprintf(`{"name":%q,"price":%q}`, it.Name, it.Price))
			}
		}
		fmt.Fprintf(w, "[%s]", strings.Join(parts, ","))
	})
	shop.HandleFunc("/upload", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, uploadPage) })
	shop.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, shopPage) })
	ln, err := net.Listen("tcp", "127.0.0.1:51627")
	if err != nil {
		return err
	}
	go http.Serve(ln, shop)

	root, err := os.MkdirTemp("/tmp", "apmcp")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	dirs := paths.Dirs{Root: root}
	if err := dirs.Ensure(); err != nil {
		return err
	}
	eng := apostate.New(dirs)
	os.MkdirAll(filepath.Dir(eng.AppPath()), 0o700)
	if err := os.Symlink(app, eng.AppPath()); err != nil {
		return err
	}
	if err := eng.Complete(); err != nil {
		return err
	}
	eng.MockKeychain = true
	st, err := store.Open(dirs.DB(), dirs.Backups())
	if err != nil {
		return err
	}
	defer st.Close()
	sec := secrets.NewMemory()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	l := launcher.New(st, sec, dirs, []engine.Engine{eng}, nil, log)
	l.Capture = true
	if headless {
		l.TestArgs = []string{"--headless=new"}
	}
	svc := &service.Service{Dirs: dirs, Store: st, Secrets: sec, Launcher: l, Log: log,
		Engines: map[string]engine.Engine{"apostate": eng}, Preferred: "apostate",
		Locale: func(context.Context, string) (string, []string, error) { return "en-US", []string{"en-US", "en"}, nil }}
	l.BeforeStart, l.AfterExit = svc.BeforeLaunch, svc.AfterExit
	ctx := context.Background()
	if _, err := svc.CreateProfile(ctx, service.NewProfile{Name: "Shop test", StartURL: "http://127.0.0.1:51627/", Tags: []string{"test"}}); err != nil {
		return err
	}
	for i := 2; i <= profiles; i++ {
		if _, err := svc.CreateProfile(ctx, service.NewProfile{Name: fmt.Sprintf("Test %d", i), StartURL: "http://127.0.0.1:51627/", Tags: []string{"test"}}); err != nil {
			return err
		}
	}
	hub := agent.NewHub(time.Minute)
	hub.Behind = l.Behind
	hub.OnActive = func(id string, on bool) { log.Info("agent", "profile", id, "driving", on) }
	srv := api.New(backend{svc}, log, port, token)
	srv.Agent = hub
	srv.Uploads, srv.Recipes = dirs.Uploads(), dirs.Recipes()
	fmt.Printf("uploads: %s\nrecipes: %s\n", dirs.Uploads(), dirs.Recipes())
	if err := srv.Start(); err != nil {
		return err
	}
	fmt.Printf("mcp at http://127.0.0.1:%d/mcp; shop at http://127.0.0.1:51627/\n", port)
	io.Copy(io.Discard, os.Stdin)
	srv.Close()
	l.StopAll()
	return nil
}
