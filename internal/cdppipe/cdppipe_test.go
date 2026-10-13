// SPDX-License-Identifier: AGPL-3.0-or-later

package cdppipe

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// fakeBrowser reads NUL-terminated commands from r and answers them from a
// queue, then emits one event, then closes.
func startFakeBrowser(t *testing.T, w, r *os.File) {
	t.Helper()
	go func() {
		br := bufio.NewReader(r)
		for {
			b, err := br.ReadBytes(0)
			if err != nil {
				return
			}
			var m wireMsg
			if json.Unmarshal(b[:len(b)-1], &m) != nil {
				continue
			}
			reply, _ := json.Marshal(wireMsg{ID: m.ID, Result: json.RawMessage(`{"ok":true}`)})
			w.Write(append(reply, 0))
			ev, _ := json.Marshal(wireMsg{Method: "X.y", Params: json.RawMessage(`{}`)})
			w.Write(append(ev, 0))
		}
	}()
}

func newPair(t *testing.T) (*Conn, *os.File) {
	t.Helper()
	appR, browserW, _ := os.Pipe() // browser writes replies, app reads
	browserR, appW, _ := os.Pipe() // browser reads commands, app writes
	startFakeBrowser(t, browserW, browserR)
	return New(appW, appR), browserW
}

func TestCallAndEvents(t *testing.T) {
	c, _ := newPair(t)
	defer c.Close()
	res, err := c.Call(context.Background(), "", "Browser.getVersion", nil)
	if err != nil || string(res) != `{"ok":true}` {
		t.Fatalf("call = %s, %v", res, err)
	}
	select {
	case ev := <-c.Events():
		if ev.Method != "X.y" {
			t.Fatalf("event = %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("no event")
	}
}

func TestCallRespectsContext(t *testing.T) {
	c, _ := newPair(t)
	defer c.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Call(ctx, "", "X", nil); err == nil {
		t.Fatal("cancelled call must fail")
	}
}

func TestCallFailsAfterClose(t *testing.T) {
	c, browserW := newPair(t)
	// Close the browser's end so the read loop sees EOF.
	browserW.Close()
	start := time.Now()
	for {
		if _, err := c.Call(context.Background(), "", "X", nil); err != nil {
			break
		}
		if time.Since(start) > time.Second {
			t.Fatal("Call did not fail within 1 s after the pipe closed")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
