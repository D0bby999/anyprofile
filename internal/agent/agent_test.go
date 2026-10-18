// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/cdppipe"
)

func init() { pause = func(_, _ time.Duration) {} }

// fakeConn answers commands from handlers by method and records them.
type fakeConn struct {
	mu       sync.Mutex
	handlers map[string]func(session string, params map[string]any) (any, error)
	calls    []string
	events   chan cdppipe.Event
	done     chan struct{}
}

func newFake() *fakeConn {
	f := &fakeConn{handlers: map[string]func(string, map[string]any) (any, error){}, events: make(chan cdppipe.Event, 16), done: make(chan struct{})}
	f.on("Target.getTargets", map[string]any{"targetInfos": []map[string]string{
		{"targetId": "AAAA1111BBBB", "type": "page", "url": "https://shop.example/", "title": "Shop"},
		{"targetId": "CCCC2222DDDD", "type": "page", "url": "https://x.invalid/probe"},
		{"targetId": "EEEE3333FFFF", "type": "service_worker", "url": "https://shop.example/sw.js"},
	}})
	f.on("Target.attachToTarget", map[string]any{"sessionId": "S1"})
	f.on("Network.enable", map[string]any{})
	f.on("Page.enable", map[string]any{})
	f.on("Target.activateTarget", map[string]any{})
	f.on("Page.getFrameTree", map[string]any{"frameTree": map[string]any{"frame": map[string]any{"id": "F", "loaderId": "L1"}}})
	f.on("Page.createIsolatedWorld", map[string]any{"executionContextId": 7})
	f.handlers["Runtime.evaluate"] = func(_ string, p map[string]any) (any, error) {
		expr := p["expression"].(string)
		switch {
		case expr == "document.visibilityState":
			return eval("visible"), nil
		case strings.Contains(expr, "location.href, title"):
			return eval(map[string]string{"url": "https://shop.example/", "title": "Shop"}), nil
		}
		return eval(nil), nil
	}
	return f
}

func eval(v any) map[string]any { return map[string]any{"result": map[string]any{"value": v}} }

func (f *fakeConn) on(method string, reply any) {
	f.handlers[method] = func(string, map[string]any) (any, error) { return reply, nil }
}

func (f *fakeConn) Call(_ context.Context, session, method string, params any) (json.RawMessage, error) {
	f.mu.Lock()
	f.calls = append(f.calls, method)
	h := f.handlers[method]
	f.mu.Unlock()
	if h == nil {
		return nil, fmt.Errorf("%s: not handled", method)
	}
	var p map[string]any
	b, _ := json.Marshal(params)
	json.Unmarshal(b, &p)
	out, err := h(session, p)
	if err != nil {
		return nil, err
	}
	return json.Marshal(out)
}

func (f *fakeConn) Subscribe(int) (<-chan cdppipe.Event, func()) { return f.events, func() {} }
func (f *fakeConn) Done() <-chan struct{}                        { return f.done }

func (f *fakeConn) called(method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if c == method {
			n++
		}
	}
	return n
}

func TestTabsSkipProbeAndWorkers(t *testing.T) {
	h := NewHub(time.Minute)
	defer h.Close()
	tabs, err := h.For("p1", newFake()).Tabs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tabs) != 1 || tabs[0].ID != "AAAA1111" || tabs[0].Title != "Shop" {
		t.Fatalf("tabs = %+v", tabs)
	}
}

func TestTabIDsArePrefixes(t *testing.T) {
	h := NewHub(time.Minute)
	defer h.Close()
	b := h.For("p1", newFake())
	ctx := context.Background()
	if _, err := b.tab(ctx, "AAAA"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"AA", "ZZZZ"} {
		if _, err := b.tab(ctx, id); err == nil || !strings.Contains(err.Error(), "agent_tab_not_found") {
			t.Fatalf("tab(%q) = %v", id, err)
		}
	}
}

// Runtime is never enabled: a page can notice it (console serialisation).
// Page is, for browser dialogs; a page script cannot see that.
func TestNeverEnablesRuntime(t *testing.T) {
	f := newFake()
	h := NewHub(time.Minute)
	defer h.Close()
	b := h.For("p1", f)
	ctx := context.Background()
	b.Tabs(ctx)
	b.ReadPage(ctx, "", "", 0, false)
	b.WaitFor(ctx, "", "", false, "shop", time.Second)
	for _, m := range []string{"Runtime.enable", "Emulation.setDeviceMetricsOverride"} {
		if f.called(m) > 0 {
			t.Fatalf("%s was called", m)
		}
	}
	// One world per document, not one per read.
	if n := f.called("Page.createIsolatedWorld"); n != 1 {
		t.Fatalf("createIsolatedWorld called %d times", n)
	}
}

func TestOnlyWebAddressesOpen(t *testing.T) {
	h := NewHub(time.Minute)
	defer h.Close()
	b := h.For("p1", newFake())
	for _, u := range []string{"file:///etc/passwd", "javascript:alert(1)", "chrome://settings", "data:text/html,x", "https://"} {
		if _, err := b.OpenTab(context.Background(), u); err == nil || !strings.Contains(err.Error(), "agent_bad_url") {
			t.Fatalf("OpenTab(%q) = %v", u, err)
		}
		if _, err := b.Navigate(context.Background(), "", u, ""); err == nil || !strings.Contains(err.Error(), "agent_bad_url") {
			t.Fatalf("Navigate(%q) = %v", u, err)
		}
	}
}

func TestActiveUntilIdle(t *testing.T) {
	h := NewHub(1500 * time.Millisecond)
	defer h.Close()
	var mu sync.Mutex
	var got []string
	h.OnActive = func(id string, on bool) { mu.Lock(); got = append(got, fmt.Sprintf("%s %v", id, on)); mu.Unlock() }
	b := h.For("p1", newFake())
	b.Tabs(context.Background())
	b.Tabs(context.Background())
	if a := h.Active(); len(a) != 1 || a[0] != "p1" {
		t.Fatalf("active = %v", a)
	}
	time.Sleep(3 * time.Second)
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(got, ",") != "p1 true,p1 false" {
		t.Fatalf("reports = %v", got)
	}
	if len(h.Active()) != 0 {
		t.Fatal("still active after idle")
	}
}

func TestClosedPipeForgetsTheBrowser(t *testing.T) {
	f := newFake()
	f.events = make(chan cdppipe.Event)
	h := NewHub(time.Minute)
	defer h.Close()
	off := make(chan bool, 1)
	h.OnActive = func(_ string, on bool) {
		if !on {
			off <- true
		}
	}
	h.For("p1", f).Tabs(context.Background())
	close(f.events)
	select {
	case <-off:
	case <-time.After(time.Second):
		t.Fatal("no inactive report when the pipe closed")
	}
	if b2 := h.For("p1", newFake()); b2 == nil || len(h.Active()) != 0 {
		t.Fatal("a new pipe must start a fresh, idle browser")
	}
}

var tree = []axNode{
	{NodeID: "1", Role: v("RootWebArea"), Name: v("Shop"), ChildIDs: []string{"2", "3", "4", "8"}},
	{NodeID: "2", Role: v("heading"), Name: v("Sale"), Properties: props("level", 2), ChildIDs: []string{"5"}},
	{NodeID: "3", Role: v("generic"), ChildIDs: []string{"6", "7"}},
	{NodeID: "4", Ignored: true, ChildIDs: []string{"9"}},
	{NodeID: "5", Role: v("StaticText"), Name: v("Sale")},
	{NodeID: "6", Role: v("textbox"), Name: v("Search   items"), Value: v("shoes"), BackendDOMNodeID: 41, Properties: props("focused", true)},
	{NodeID: "7", Role: v("checkbox"), Name: v("In stock"), BackendDOMNodeID: 42, Properties: props("checked", "true")},
	{NodeID: "8", Role: v("generic"), Name: v("Card"), BackendDOMNodeID: 43, Properties: props("focusable", true)},
	{NodeID: "9", Role: v("link"), Name: v("Home"), BackendDOMNodeID: 44, ChildIDs: []string{"10"}},
	{NodeID: "10", Role: v("StaticText"), Name: v("Home")},
}

func v(x any) *axValue { b, _ := json.Marshal(x); return &axValue{Value: b} }

func props(name string, val any) []struct {
	Name  string  `json:"name"`
	Value axValue `json:"value"`
} {
	return []struct {
		Name  string  `json:"name"`
		Value axValue `json:"value"`
	}{{Name: name, Value: *v(val)}}
}

func TestOutline(t *testing.T) {
	got := outline(tree, "1", MaxText, nil, nil)
	want := `- heading "Sale" [level=2]
- textbox "Search items" [ref=e41] [focused] [value="shoes"]
- checkbox "In stock" [ref=e42] [checked]
- link "Home" [ref=e44]
- generic "Card" [ref=e43]
`
	if got != want {
		t.Fatalf("outline:\n%s\nwant:\n%s", got, want)
	}
	if cut := outline(tree, "1", 10, nil, nil); !strings.Contains(cut, "cut at 10 characters") {
		t.Fatalf("cut outline:\n%s", cut)
	}
}

func TestResolveRef(t *testing.T) {
	b := &Browser{}
	f1, f2 := &frameSess{id: "F1"}, &frameSess{id: "F2", oopif: true}
	tb := &tab{frames: []*frameSess{f1, f2}}
	if n, f, err := b.resolve(tb, "e42"); n != 42 || f != nil || err != nil {
		t.Fatal(n, f, err)
	}
	if n, f, err := b.resolve(tb, "e7f2"); n != 7 || f != f2 || err != nil {
		t.Fatal(n, f, err)
	}
	if tb.refFor(7, f2) != "e7f2" || tb.refFor(9, nil) != "e9" {
		t.Fatal(tb.refFor(7, f2), tb.refFor(9, nil))
	}
	for _, bad := range []string{"42", "e", "e-1", "x42", "", "e7f", "e7f0", "e7fx"} {
		if _, _, err := b.resolve(tb, bad); err == nil {
			t.Fatalf("resolve(%q) passed", bad)
		}
	}
	if _, _, err := b.resolve(tb, "e7f3"); err == nil || !strings.Contains(err.Error(), "agent_") {
		t.Fatalf("a frame not reached: %v", err)
	}
}

func TestNetLog(t *testing.T) {
	l := newNetLog(3)
	ev := func(m, js string) { l.add(m, json.RawMessage(js)) }
	ev("Network.requestWillBeSent", `{"requestId":"1","type":"Document","request":{"url":"https://a/","method":"GET"}}`)
	ev("Network.requestWillBeSent", `{"requestId":"1","type":"Document","request":{"url":"https://a/home","method":"GET"}}`) // redirect
	ev("Network.responseReceived", `{"requestId":"1","type":"Document","response":{"status":200,"mimeType":"text/html"}}`)
	ev("Network.loadingFinished", `{"requestId":"1","encodedDataLength":512}`)
	ev("Network.requestWillBeSent", `{"requestId":"d","request":{"url":"data:image/png;base64,xx","method":"GET"}}`)
	for i := 2; i <= 4; i++ {
		ev("Network.requestWillBeSent", fmt.Sprintf(`{"requestId":"%d","type":"Fetch","request":{"url":"https://a/api/%d","method":"POST"}}`, i, i))
	}
	ev("Network.loadingFailed", `{"requestId":"4","errorText":"net::ERR_FAILED"}`)
	all := l.list("", nil, 50)
	if len(all) != 3 || all[0].ID != "2" || all[2].Failed != "net::ERR_FAILED" {
		t.Fatalf("list = %+v", all)
	}
	if _, ok := l.byID["1"]; ok {
		t.Fatal("the oldest request must be dropped")
	}
	if got := l.list("api/3", []string{"fetch"}, 50); len(got) != 1 || got[0].Method != "POST" {
		t.Fatalf("filtered = %+v", got)
	}
	if got := l.list("", nil, 1); len(got) != 1 || got[0].ID != "4" {
		t.Fatalf("limit keeps the newest: %+v", got)
	}
}

func TestBinaryBodiesAreNotReturned(t *testing.T) {
	f := newFake()
	f.on("Network.getResponseBody", map[string]any{"body": "iVBORw0KGgo=", "base64Encoded": true})
	h := NewHub(time.Minute)
	defer h.Close()
	b := h.For("p1", f)
	ctx := context.Background()
	tb, _ := b.tab(ctx, "")
	b.mu.Lock()
	tb.net.add("Network.requestWillBeSent", json.RawMessage(`{"requestId":"9","request":{"url":"https://a/x.png","method":"GET"}}`))
	tb.net.add("Network.responseReceived", json.RawMessage(`{"requestId":"9","response":{"status":200,"mimeType":"image/png"}}`))
	b.mu.Unlock()
	out, err := b.ResponseBody(ctx, "", "9", 0)
	if err != nil || !strings.Contains(out, "binary image/png") || strings.Contains(out, "iVBOR") {
		t.Fatalf("body = %q, %v", out, err)
	}
	if _, err := b.ResponseBody(ctx, "", "nope", 0); err == nil || !strings.Contains(err.Error(), "agent_request_not_found") {
		t.Fatalf("unknown request: %v", err)
	}
}

func TestKeys(t *testing.T) {
	if k, ok := keyFor('a'); !ok || k.code != "KeyA" || k.vk != 65 || k.text != "a" {
		t.Fatalf("a = %+v", k)
	}
	if k, ok := keyFor('7'); !ok || k.code != "Digit7" {
		t.Fatalf("7 = %+v", k)
	}
	if _, ok := keyFor('ă'); ok {
		t.Fatal("non-ASCII goes through insertText")
	}
	f := newFake()
	var sent []map[string]any
	f.handlers["Input.dispatchKeyEvent"] = func(_ string, p map[string]any) (any, error) { sent = append(sent, p); return map[string]any{}, nil }
	h := NewHub(time.Minute)
	defer h.Close()
	b := h.For("p1", f)
	if _, err := b.PressKey(context.Background(), "", "Control+Shift+k"); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 2 || sent[0]["type"] != "rawKeyDown" || sent[0]["modifiers"] != float64(modCtrl|modShift) || sent[1]["type"] != "keyUp" {
		t.Fatalf("sent = %v", sent)
	}
	for _, bad := range []string{"Hyper+a", "F13"} {
		if _, err := b.PressKey(context.Background(), "", bad); err == nil {
			t.Fatalf("PressKey(%q) passed", bad)
		}
	}
}

func TestShrink(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2560, 1600))
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, nil)
	out, err := shrink(buf.Bytes(), MaxShotWidth)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := jpeg.DecodeConfig(bytes.NewReader(out))
	if cfg.Width != 1280 || cfg.Height != 800 {
		t.Fatalf("shrunk to %dx%d", cfg.Width, cfg.Height)
	}
}
