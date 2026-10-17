// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
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
	"github.com/D0bby999/anyprofile/internal/cdppipe"
)

// pipe answers the few DevTools commands a snapshot needs.
type pipe struct{ done chan struct{} }

func (p *pipe) Call(_ context.Context, _, method string, params any) (json.RawMessage, error) {
	var out any = map[string]any{}
	switch method {
	case "Target.getTargets":
		out = map[string]any{"targetInfos": []map[string]string{{"targetId": "T1234567890", "type": "page", "url": "https://shop.example/", "title": "Shop"}}}
	case "Target.attachToTarget":
		out = map[string]any{"sessionId": "S"}
	case "Page.getFrameTree":
		out = map[string]any{"frameTree": map[string]any{"frame": map[string]any{"id": "F", "loaderId": "L"}}}
	case "Page.createIsolatedWorld":
		out = map[string]any{"executionContextId": 3}
	case "Runtime.evaluate":
		expr := params.(map[string]any)["expression"].(string)
		v := any("visible")
		if strings.Contains(expr, "title") {
			v = map[string]string{"url": "https://shop.example/", "title": "Shop"}
		}
		out = map[string]any{"result": map[string]any{"value": v}}
	case "Accessibility.getFullAXTree":
		out = map[string]any{"nodes": []map[string]any{
			{"nodeId": "1", "role": map[string]any{"value": "RootWebArea"}, "childIds": []string{"2"}},
			{"nodeId": "2", "role": map[string]any{"value": "button"}, "name": map[string]any{"value": "Go"}, "backendDOMNodeId": 5},
		}}
	}
	return json.Marshal(out)
}

func (p *pipe) Subscribe(int) (<-chan cdppipe.Event, func()) {
	ch := make(chan cdppipe.Event)
	return ch, func() {}
}
func (p *pipe) Done() <-chan struct{} { return p.done }

func (f *fake) Pipe(id string) (agent.Conn, bool) {
	for _, p := range f.profiles {
		if p.ID == id && p.State == "running" && !f.adopted {
			return f.pipe, true
		}
	}
	return nil, false
}

// rewrite sends every request as a local client of the API would.
type rewrite struct {
	token, origin string
}

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Host = "127.0.0.1:51525"
	req.Header.Set("Authorization", "Bearer "+r.token)
	if r.origin != "" {
		req.Header.Set("Origin", r.origin)
	}
	return http.DefaultTransport.RoundTrip(req)
}

var logBuf syncBuffer

type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}
func (s *syncBuffer) String() string { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

func mcpSetup(t *testing.T) (*fake, *Server, *httptest.Server) {
	t.Helper()
	f := &fake{pipe: &pipe{done: make(chan struct{})}, profiles: []Profile{
		{ID: "p1", Serial: 1, Name: "Kênh", Sites: []string{"tiktok"}, Tags: []string{"vn"}, State: "stopped"},
		{ID: "p2", Serial: 2, Name: "Shop", Sites: []string{"shopee"}, Tags: []string{}, State: "stopped"},
		{ID: "p3", Serial: 3, Name: "shop", Sites: []string{}, Tags: []string{}, State: "stopped"},
	}}
	s := New(f, slog.New(slog.NewTextHandler(&logBuf, nil)), port, token)
	s.Agent = agent.NewHub(time.Minute)
	t.Cleanup(s.Agent.Close)
	s.Uploads, s.Recipes = t.TempDir(), t.TempDir()
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return f, s, srv
}

func connect(t *testing.T, srv *httptest.Server, rt rewrite) (*mcp.ClientSession, error) {
	t.Helper()
	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	return c.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: srv.URL + "/mcp", HTTPClient: &http.Client{Transport: rt}, DisableStandaloneSSE: true,
	}, nil)
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String(), res.IsError
}

func TestMCPRefusesWebPagesAndStrangers(t *testing.T) {
	_, _, srv := mcpSetup(t)
	for _, rt := range []rewrite{{token: "wrong"}, {token: token, origin: "https://evil.example"}} {
		if cs, err := connect(t, srv, rt); err == nil {
			cs.Close()
			t.Fatalf("connected with %+v", rt)
		}
	}
}

func TestMCPTools(t *testing.T) {
	_, _, srv := mcpSetup(t)
	cs, err := connect(t, srv, rewrite{token: token})
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	init := cs.InitializeResult()
	if init.ServerInfo.Name != "anyprofile" || !strings.Contains(init.Instructions, "untrusted") {
		t.Fatalf("initialize = %+v", init)
	}
	list, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tl := range list.Tools {
		names = append(names, tl.Name)
		if tl.Name == "snapshot" {
			// The shared profile and tab fields reach the schema; only profile is required.
			b, _ := json.Marshal(tl.InputSchema)
			var sc struct {
				Properties map[string]any `json:"properties"`
				Required   []string       `json:"required"`
			}
			json.Unmarshal(b, &sc)
			if sc.Properties["tab"] == nil || sc.Properties["ref"] == nil || !slices.Equal(sc.Required, []string{"profile"}) {
				t.Fatalf("snapshot schema = %s", b)
			}
		}
	}
	slices.Sort(names)
	want := []string{"click", "close_profile", "close_tab", "list_profiles", "list_tabs", "list_uploads", "navigate", "network_requests",
		"open_profile", "open_tab", "press_key", "read_page", "recipe", "response_body", "screenshot", "scroll", "snapshot", "type",
		"upload_file", "wait_for"}
	if !slices.Equal(names, want) {
		t.Fatalf("tools = %v", names)
	}
}

func TestMCPOpensAProfileAndReadsIt(t *testing.T) {
	f, s, srv := mcpSetup(t)
	cs, err := connect(t, srv, rewrite{token: token})
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	out, isErr := call(t, cs, "list_profiles", map[string]any{"tag": "vn"})
	if isErr || !strings.Contains(out, `p1 #1 "Kênh" stopped sites=tiktok tags=vn`) || strings.Contains(out, "Shop") {
		t.Fatalf("list_profiles = %q", out)
	}
	if out, isErr = call(t, cs, "snapshot", map[string]any{"profile": "Kênh"}); !isErr || !strings.Contains(out, "agent_not_open") {
		t.Fatalf("snapshot of a closed profile = %q", out)
	}
	if out, isErr = call(t, cs, "open_profile", map[string]any{"profile": "#1"}); isErr || !strings.Contains(out, `T1234567 (current) "Shop"`) {
		t.Fatalf("open_profile = %q", out)
	}
	if len(f.started) != 1 || f.started[0] != "p1" || f.devtools {
		t.Fatalf("started %v devtools=%v; the agent needs no DevTools port", f.started, f.devtools)
	}
	if out, isErr = call(t, cs, "snapshot", map[string]any{"profile": "p1"}); isErr || !strings.Contains(out, `- button "Go" [ref=e5]`) {
		t.Fatalf("snapshot = %q", out)
	}
	if got := s.Agent.Active(); !slices.Equal(got, []string{"p1"}) {
		t.Fatalf("active = %v", got)
	}
	if out, isErr = call(t, cs, "close_profile", map[string]any{"profile": "p1"}); isErr || len(f.stopped) != 1 {
		t.Fatalf("close_profile = %q, stopped %v", out, f.stopped)
	}
	// The log names profiles by id, never by what the agent typed.
	if log := logBuf.String(); !strings.Contains(log, "tool=snapshot profile=p1") || strings.Contains(log, "Kênh") || strings.Contains(log, `profile="#1"`) {
		t.Fatalf("log:\n%s", log)
	}
}

func TestMCPProfileLookup(t *testing.T) {
	f, s, srv := mcpSetup(t)
	cs, err := connect(t, srv, rewrite{token: token})
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	if out, isErr := call(t, cs, "open_profile", map[string]any{"profile": "SHOP"}); !isErr || !strings.Contains(out, "agent_ambiguous") || !strings.Contains(out, "p2 (#2)") {
		t.Fatalf("ambiguous name = %q", out)
	}
	if out, isErr := call(t, cs, "open_profile", map[string]any{"profile": "nobody"}); !isErr || !strings.Contains(out, "api_not_found") {
		t.Fatalf("unknown = %q", out)
	}
	// Open before the app restarted: running, but no pipe to drive it with.
	f.profiles[1].State, f.adopted = "running", true
	if out, isErr := call(t, cs, "open_profile", map[string]any{"profile": "p2"}); !isErr || !strings.Contains(out, "agent_no_pipe") {
		t.Fatalf("adopted = %q", out)
	}
	if p, err := s.profile(" #2 "); err != nil || p.ID != "p2" {
		t.Fatal(p, err)
	}
	if len(f.started) != 0 {
		t.Fatalf("nothing should start: %v", f.started)
	}
}

func TestClaudeCommand(t *testing.T) {
	got := ClaudeCommand(51525, "ab12")
	want := `claude mcp add --transport http anyprofile http://127.0.0.1:51525/mcp --header "Authorization: Bearer ab12"`
	if got != want {
		t.Fatalf("command = %s", got)
	}
}

func TestMCPUploadsStayInTheFolder(t *testing.T) {
	_, s, srv := mcpSetup(t)
	cs, err := connect(t, srv, rewrite{token: token})
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	if out, isErr := call(t, cs, "list_uploads", nil); isErr || !strings.Contains(out, "empty") {
		t.Fatalf("empty folder = %q", out)
	}
	os.WriteFile(filepath.Join(s.Uploads, "clip.mp4"), []byte("1234"), 0o600)
	if out, isErr := call(t, cs, "list_uploads", nil); isErr || out != "clip.mp4 (4 bytes)\n" {
		t.Fatalf("list_uploads = %q", out)
	}
	// Refused before the profile is even looked at.
	out, isErr := call(t, cs, "upload_file", map[string]any{"profile": "p1", "ref": "e5", "files": []string{"/etc/hosts"}})
	if !isErr || !strings.Contains(out, "agent_upload_outside") {
		t.Fatalf("upload outside = %q", out)
	}
}

func TestMCPRecipes(t *testing.T) {
	_, s, srv := mcpSetup(t)
	cs, err := connect(t, srv, rewrite{token: token})
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	ctx := context.Background()
	if out, _ := call(t, cs, "recipe", nil); !strings.Contains(out, "No recipes yet") {
		t.Fatalf("no recipes = %q", out)
	}
	os.WriteFile(filepath.Join(s.Recipes, "yt.md"), []byte("---\nname: youtube-upload\ndescription: Upload a video\nversion: \"2\"\narguments:\n  - name: file\n    description: file in uploads\n    required: true\n---\nUpload {{file}}; stop before Publish.\n"), 0o600)
	os.WriteFile(filepath.Join(s.Recipes, "bad.md"), []byte("no front matter"), 0o600)

	out, _ := call(t, cs, "recipe", nil)
	if !strings.Contains(out, "- youtube-upload: Upload a video (arguments: file*") || !strings.Contains(out, "Not loaded: anyprofile:recipe_invalid: bad.md") {
		t.Fatalf("recipe list = %q", out)
	}
	out, isErr := call(t, cs, "recipe", map[string]any{"name": "youtube-upload"})
	if isErr || !strings.Contains(out, "Recipe youtube-upload v2") || !strings.Contains(out, "Upload (ask the user: file in uploads)") {
		t.Fatalf("recipe without arguments = %q", out)
	}
	out, _ = call(t, cs, "recipe", map[string]any{"name": "youtube-upload", "arguments": map[string]string{"file": "clip.mp4"}})
	if !strings.Contains(out, "Upload clip.mp4; stop before Publish.") {
		t.Fatalf("recipe = %q", out)
	}

	// The same file is a prompt, picked up without a restart.
	ps, err := cs.ListPrompts(ctx, nil)
	if err != nil || len(ps.Prompts) != 1 || ps.Prompts[0].Name != "youtube-upload" || !ps.Prompts[0].Arguments[0].Required {
		t.Fatalf("prompts = %+v, %v", ps, err)
	}
	got, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: "youtube-upload", Arguments: map[string]string{"file": "clip.mp4"}})
	if err != nil || !strings.Contains(got.Messages[0].Content.(*mcp.TextContent).Text, "Upload clip.mp4") {
		t.Fatalf("prompt = %+v, %v", got, err)
	}
	if _, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: "youtube-upload"}); err == nil {
		t.Fatal("a prompt without its required argument must fail")
	}
	// Removed from the folder: withdrawn.
	os.Remove(filepath.Join(s.Recipes, "yt.md"))
	if ps, _ := cs.ListPrompts(ctx, nil); len(ps.Prompts) != 0 {
		t.Fatalf("prompts after removal = %+v", ps.Prompts)
	}
}
