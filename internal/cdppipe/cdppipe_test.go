// SPDX-License-Identifier: AGPL-3.0-or-later

package cdppipe

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
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
	// Two subscribers both get the event; neither takes it from the other.
	a, cancelA := c.Subscribe(4)
	defer cancelA()
	b, cancelB := c.Subscribe(4)
	defer cancelB()
	res, err := c.Call(context.Background(), "", "Browser.getVersion", nil)
	if err != nil || string(res) != `{"ok":true}` {
		t.Fatalf("call = %s, %v", res, err)
	}
	for _, ch := range []<-chan Event{a, b} {
		select {
		case ev := <-ch:
			if ev.Method != "X.y" {
				t.Fatalf("event = %+v", ev)
			}
		case <-time.After(time.Second):
			t.Fatal("no event")
		}
	}
}

func TestCancelClosesOnlyThatSubscriber(t *testing.T) {
	c, _ := newPair(t)
	defer c.Close()
	a, cancelA := c.Subscribe(4)
	b, cancelB := c.Subscribe(4)
	defer cancelB()
	cancelA()
	cancelA() // twice is harmless
	if _, ok := <-a; ok {
		t.Fatal("a cancelled subscription must be closed")
	}
	c.Call(context.Background(), "", "X", nil)
	select {
	case <-b:
	case <-time.After(time.Second):
		t.Fatal("the other subscriber stopped getting events")
	}
}

// A subscriber that never reads must not hold up replies.
func TestSlowSubscriberDoesNotBlockCalls(t *testing.T) {
	c, _ := newPair(t)
	defer c.Close()
	_, cancel := c.Subscribe(1)
	defer cancel()
	for i := 0; i < 20; i++ {
		ctx, done := context.WithTimeout(context.Background(), time.Second)
		if _, err := c.Call(ctx, "", "X", nil); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		done()
	}
}

func TestSubscriptionsCloseWithThePipe(t *testing.T) {
	c, browserW := newPair(t)
	ch, cancel := c.Subscribe(4)
	defer cancel()
	browserW.Close()
	deadline := time.After(time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				late, _ := c.Subscribe(4)
				if _, ok := <-late; ok {
					t.Fatal("a subscription after the pipe closed must be closed")
				}
				return
			}
		case <-deadline:
			t.Fatal("subscription not closed with the pipe")
		}
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

// echoBrowser answers each command with its own params, so a caller can tell
// whether it got its own reply and whether its command arrived whole.
func echoBrowser(w, r *os.File) {
	go func() {
		br := bufio.NewReaderSize(r, 1<<20)
		for {
			b, err := br.ReadBytes(0)
			if err != nil {
				return
			}
			var m wireMsg
			if json.Unmarshal(b[:len(b)-1], &m) != nil {
				reply, _ := json.Marshal(wireMsg{ID: -1})
				w.Write(append(reply, 0))
				continue
			}
			reply, _ := json.Marshal(wireMsg{ID: m.ID, Result: m.Params})
			w.Write(append(reply, 0))
		}
	}()
}

func TestConcurrentCallsStayWhole(t *testing.T) {
	appR, browserW, _ := os.Pipe()
	browserR, appW, _ := os.Pipe()
	echoBrowser(browserW, browserR)
	c := New(appW, appR)
	defer c.Close()
	var wg sync.WaitGroup
	errs := make(chan string, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Larger than a pipe buffer, so an unlocked write could split.
			want := strings.Repeat(string(rune('a'+i%26)), 70000)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			res, err := c.Call(ctx, "", "Echo", map[string]string{"v": want})
			var got struct{ V string }
			if err != nil || json.Unmarshal(res, &got) != nil || got.V != want {
				errs <- "call " + string(rune('0'+i%10)) + " got the wrong reply"
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}
