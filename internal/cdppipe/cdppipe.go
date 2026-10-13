// SPDX-License-Identifier: AGPL-3.0-or-later

// Package cdppipe speaks the DevTools protocol over the pipe pair that
// --remote-debugging-pipe gives Chromium: NUL-terminated JSON messages.
package cdppipe

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Event is a protocol event (no id).
type Event struct {
	Method    string
	SessionID string
	Params    json.RawMessage
}

type wireMsg struct {
	ID        int             `json:"id,omitempty"`
	Method    string          `json:"method,omitempty"`
	Params    json.RawMessage `json:"params,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     json.RawMessage `json:"error,omitempty"`
}

// Conn owns the app's ends of the command and reply pipes. The browser keeps
// its own copies (fds 7 and 8), so closing Conn never kills the browser.
type Conn struct {
	w      *os.File
	mu     sync.Mutex
	next   int
	wait   map[int]chan wireMsg
	events chan Event
	closed chan struct{}
}

// New starts reading r in the background. w carries commands to the browser.
func New(w, r *os.File) *Conn {
	c := &Conn{w: w, wait: map[int]chan wireMsg{}, events: make(chan Event, 64), closed: make(chan struct{})}
	go c.readLoop(r)
	return c
}

func (c *Conn) readLoop(r *os.File) {
	br := bufio.NewReader(r)
	for {
		b, err := br.ReadBytes(0)
		if err != nil {
			close(c.events)
			close(c.closed)
			return
		}
		var m wireMsg
		if json.Unmarshal(bytes.TrimSuffix(b, []byte{0}), &m) != nil {
			continue
		}
		if m.ID != 0 {
			c.mu.Lock()
			ch := c.wait[m.ID]
			delete(c.wait, m.ID)
			c.mu.Unlock()
			if ch != nil {
				ch <- m
			}
			continue
		}
		// The reader must never block, or the reply pipe could fill up. Drop
		// events when the consumer is slow rather than blocking here.
		select {
		case c.events <- Event{Method: m.Method, SessionID: m.SessionID, Params: m.Params}:
		default:
		}
	}
}

// Call sends one command (session "" means the browser) and waits for its
// reply or ctx. A protocol error becomes a Go error carrying the method name.
func (c *Conn) Call(ctx context.Context, session, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	c.next++
	id := c.next
	ch := make(chan wireMsg, 1)
	c.wait[id] = ch
	c.mu.Unlock()
	p, _ := json.Marshal(params)
	b, _ := json.Marshal(wireMsg{ID: id, Method: method, Params: p, SessionID: session})
	if _, err := c.w.Write(append(b, 0)); err != nil {
		return nil, err
	}
	select {
	case m := <-ch:
		if len(m.Error) > 0 {
			return nil, fmt.Errorf("%s: %s", method, m.Error)
		}
		return m.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.closed:
		return nil, fmt.Errorf("%s: pipe closed", method)
	}
}

// Events delivers protocol events; it is closed when the browser side goes away.
func (c *Conn) Events() <-chan Event { return c.events }

// Close closes the app's ends. The browser keeps its own copies (fds 7 and 8).
func (c *Conn) Close() error { return c.w.Close() }
