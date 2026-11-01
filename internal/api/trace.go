// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// traceLine is one MCP tool call. It never carries page content or typed text,
// only what the tool was asked to do and how it ended.
type traceLine struct {
	TS   string `json:"ts"`
	Tool string `json:"tool"`
	Who  string `json:"who,omitempty"`
	MS   int64  `json:"ms"`
	OK   bool   `json:"ok"`
	Err  string `json:"err,omitempty"`
	Shot string `json:"shot,omitempty"`
}

// trace appends one line to Traces/<date>.jsonl. Tracing must never fail a
// tool call: a missing folder or a full disk is ignored.
func (s *Server) trace(tool, who string, ms int64, err error, shot string) {
	if s.Traces == "" {
		return
	}
	line := traceLine{TS: time.Now().UTC().Format(time.RFC3339Nano), Tool: tool, Who: who, MS: ms, OK: err == nil, Shot: shot}
	if err != nil {
		msg := err.Error()
		if len(msg) > 500 {
			msg = msg[:500] + "…"
		}
		line.Err = msg
	}
	b, jerr := json.Marshal(line)
	if jerr != nil {
		return
	}
	s.traceMu.Lock()
	defer s.traceMu.Unlock()
	if mkerr := os.MkdirAll(s.Traces, 0o700); mkerr != nil {
		return
	}
	f, ferr := os.OpenFile(filepath.Join(s.Traces, "mcp-"+time.Now().Format("2006-01-02")+".jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if ferr != nil {
		return
	}
	_, _ = f.Write(append(b, '\n'))
	_ = f.Close()
}

// traceShot saves the surface as a JPEG on a failed tool call. A screenshot
// that cannot be taken is not an error: the trace line simply has no shot.
func (s *Server) traceShot(tool, who string, browser bool) string {
	if s.Traces == "" || who == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var img []byte
	var err error
	if browser {
		_, b, e := s.browser(who)
		if e != nil {
			return ""
		}
		img, err = b.Screenshot(ctx, "")
	} else {
		d, e := s.device(who)
		if e != nil {
			return ""
		}
		c, e := s.control(d)
		if e != nil {
			return ""
		}
		img, err = c.Screenshot(ctx)
	}
	if err != nil || len(img) == 0 {
		return ""
	}
	if err := os.MkdirAll(s.Traces, 0o700); err != nil {
		return ""
	}
	name := fmt.Sprintf("%s-%s-%d.jpg", time.Now().Format("2006-01-02"), tool, time.Now().UnixNano())
	path := filepath.Join(s.Traces, name)
	if err := os.WriteFile(path, img, 0o600); err != nil {
		return ""
	}
	return path
}
