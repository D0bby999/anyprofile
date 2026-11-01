// SPDX-License-Identifier: AGPL-3.0-or-later

package adb

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Session is one long-lived `adb shell`. Every Run writes a command and a
// unique marker; the marker carries the command's exit status. Spawning adb
// once per tap is the main cost of the agent path, so the control driver
// keeps one of these per running device.
type Session struct {
	cmd   *exec.Cmd
	in    io.WriteCloser
	out   *bufio.Reader
	token string
	done  chan struct{}
	mu    sync.Mutex
	dead  bool
}

// OpenShell starts the persistent shell. The caller owns Close.
func (a Client) OpenShell(ctx context.Context) (*Session, error) {
	// A persistent shell must outlive the command that opened it. CommandContext
	// would kill it when the first action's per-command timeout fires.
	_ = ctx
	cmd := exec.Command(a.Bin, "-s", a.Serial(), "shell")
	cmd.Env = a.Env()
	cmd.SysProcAttr = hidden()
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	s := &Session{cmd: cmd, in: in, out: bufio.NewReader(out), token: newToken(), done: make(chan struct{})}
	go func() { _ = cmd.Wait(); close(s.done) }()
	return s, nil
}

func newToken() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return "__AP_" + hex.EncodeToString(b[:]) + "__"
}

// Run executes one shell command and returns its stdout without the marker.
func (s *Session) Run(ctx context.Context, line string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dead {
		return "", errors.New("adb shell session is closed")
	}
	// printf is part of the Android shell, not a command the caller controls.
	if _, err := fmt.Fprintf(s.in, "%s; printf '\\n%s:%%s\\n' \"$?\"\n", line, s.token); err != nil {
		s.dead = true
		return "", err
	}
	type result struct {
		out string
		err error
	}
	ch := make(chan result, 1)
	go func() {
		var b strings.Builder
		for {
			line, err := s.out.ReadString('\n')
			if err != nil {
				ch <- result{err: err}
				return
			}
			line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
			if strings.HasPrefix(line, s.token+":") {
				status := strings.TrimPrefix(line, s.token+":")
				out := strings.TrimSpace(b.String())
				if status != "0" && status != "" {
					ch <- result{out: out, err: fmt.Errorf("shell exit %s", status)}
					return
				}
				ch <- result{out: out}
				return
			}
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}()
	select {
	case <-ctx.Done():
		s.dead = true
		_ = s.in.Close()
		return "", ctx.Err()
	case r := <-ch:
		if r.err != nil && !strings.HasPrefix(r.err.Error(), "shell exit ") {
			s.dead = true
		}
		return r.out, r.err
	}
}

// Close ends the shell; it is safe to call more than once.
func (s *Session) Close() {
	s.mu.Lock()
	if s.dead {
		s.mu.Unlock()
		return
	}
	s.dead = true
	_, _ = io.WriteString(s.in, "exit\n")
	_ = s.in.Close()
	s.mu.Unlock()
	select {
	case <-s.done:
	case <-time.After(time.Second):
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
		<-s.done
	}
}
