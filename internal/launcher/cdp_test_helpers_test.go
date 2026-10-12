// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// cdp is a minimal DevTools client for tests. It sends no Origin header,
// which is what lets it connect without --remote-allow-origins.
type cdp struct {
	c  *websocket.Conn
	id atomic.Int64
}

func dialPage(t *testing.T, port int) *cdp {
	t.Helper()
	var targets []struct{ Type, WebSocketDebuggerURL string }
	for i := 0; i < 50; i++ {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
		if err == nil {
			json.NewDecoder(resp.Body).Decode(&targets)
			resp.Body.Close()
		}
		for _, tg := range targets {
			if tg.Type == "page" {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				c, _, err := websocket.Dial(ctx, tg.WebSocketDebuggerURL, nil)
				if err != nil {
					t.Fatalf("dial devtools: %v", err)
				}
				c.SetReadLimit(16 << 20)
				return &cdp{c: c}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("no page target")
	return nil
}

func (d *cdp) call(t *testing.T, method string, params any) json.RawMessage {
	t.Helper()
	id := d.id.Add(1)
	msg, _ := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := d.c.Write(ctx, websocket.MessageText, msg); err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	for {
		_, b, err := d.c.Read(ctx)
		if err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		var r struct {
			ID     int64           `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		json.Unmarshal(b, &r)
		if r.ID == id {
			if r.Error != nil {
				t.Fatalf("%s: %s", method, r.Error)
			}
			return r.Result
		}
	}
}

// eval runs an expression (awaiting promises) and returns its string value.
func (d *cdp) eval(t *testing.T, expr string) string {
	res := d.call(t, "Runtime.evaluate", map[string]any{"expression": expr, "awaitPromise": true, "returnByValue": true})
	var v struct {
		Result struct{ Value any } `json:"result"`
	}
	json.Unmarshal(res, &v)
	return fmt.Sprint(v.Result.Value)
}

func (d *cdp) navigate(t *testing.T, url string) {
	d.call(t, "Page.navigate", map[string]any{"url": url})
	for i := 0; i < 100; i++ {
		if d.eval(t, "document.readyState") == "complete" && d.eval(t, "location.href") == url {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("navigation to %s did not finish", url)
}

func (d *cdp) close() { d.c.Close(websocket.StatusNormalClosure, "") }
