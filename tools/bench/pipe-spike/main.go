// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

// pipe-spike checks, on a throwaway profile, that the Fury core
//
//  1. accepts its config and cookie key on fds 5 and 6,
//
//  2. serves the DevTools protocol over --remote-debugging-pipe (fds 3 and 4),
//
//  3. lets a fingerprint probe run in a locally fulfilled https page,
//
//  4. keeps running after the pipe closes, with no TCP port open,
//
//  5. reports the same fingerprint on a second launch of the same profile.
//
//     go run ./tools/bench/pipe-spike [-app <Fury.app>]
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/engine/fury"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/store"
)

const probeOrigin = "https://fp.anyprofile.invalid/"

const probeJS = `(async () => {
  const c = document.createElement('canvas'); c.width = 280; c.height = 60;
  const x = c.getContext('2d'); x.textBaseline = 'top'; x.font = '16px Arial';
  x.fillStyle = '#f60'; x.fillRect(100, 1, 62, 20);
  x.fillStyle = '#069'; x.fillText('AnyProfile fp 1.0', 2, 15);
  x.fillStyle = 'rgba(102,204,0,0.7)'; x.fillText('AnyProfile fp 1.0', 4, 17);
  const hash = async (s) => Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',
    new TextEncoder().encode(s)))).map((b) => b.toString(16).padStart(2, '0')).join('').slice(0, 16);
  const gl = document.createElement('canvas').getContext('webgl');
  const dbg = gl && gl.getExtension('WEBGL_debug_renderer_info');
  return { canvas: await hash(c.toDataURL()),
    webglVendor: dbg ? gl.getParameter(dbg.UNMASKED_VENDOR_WEBGL) : '',
    webglRenderer: dbg ? gl.getParameter(dbg.UNMASKED_RENDERER_WEBGL) : '',
    ua: navigator.userAgent, platform: navigator.platform,
    hwc: navigator.hardwareConcurrency, mem: navigator.deviceMemory,
    screen: screen.width + 'x' + screen.height,
    tz: Intl.DateTimeFormat().resolvedOptions().timeZone, langs: navigator.languages.join(','),
    secure: isSecureContext };
})()`

type msg struct {
	ID        int             `json:"id,omitempty"`
	Method    string          `json:"method,omitempty"`
	Params    json.RawMessage `json:"params,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     json.RawMessage `json:"error,omitempty"`
}

type client struct {
	w      *os.File
	mu     sync.Mutex
	next   int
	wait   map[int]chan msg
	events chan msg
}

func newClient(w, r *os.File) *client {
	c := &client{w: w, wait: map[int]chan msg{}, events: make(chan msg, 64)}
	go func() {
		br := bufio.NewReader(r)
		for {
			b, err := br.ReadBytes(0)
			if err != nil {
				close(c.events)
				return
			}
			var m msg
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
			} else {
				c.events <- m
			}
		}
	}()
	return c
}

func (c *client) call(session, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	c.next++
	id := c.next
	ch := make(chan msg, 1)
	c.wait[id] = ch
	c.mu.Unlock()
	p, _ := json.Marshal(params)
	b, _ := json.Marshal(msg{ID: id, Method: method, Params: p, SessionID: session})
	if _, err := c.w.Write(append(b, 0)); err != nil {
		return nil, err
	}
	select {
	case m := <-ch:
		if len(m.Error) > 0 {
			return nil, fmt.Errorf("%s: %s", method, m.Error)
		}
		return m.Result, nil
	case <-time.After(15 * time.Second):
		return nil, fmt.Errorf("%s: timeout", method)
	}
}

func main() {
	home, _ := os.UserHomeDir()
	app := flag.String("app", filepath.Join(home, "Library/Application Support/AnyProfile/engines/fury", fury.Version, "Fury.app"), "verified Fury.app")
	flag.Parse()
	if err := run(*app); err != nil {
		fmt.Println("SPIKE FAILED:", err)
		os.Exit(1)
	}
}

func run(app string) error {
	root, err := os.MkdirTemp("/tmp", "ap-spike")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	dirs := paths.Dirs{Root: root}
	if err := dirs.Ensure(); err != nil {
		return err
	}
	os.MkdirAll(dirs.Engine("fury", fury.Version), 0o700)
	if err := os.Symlink(app, filepath.Join(dirs.Engine("fury", fury.Version), "Fury.app")); err != nil {
		return err
	}
	cfgBin, _ := filepath.Abs("tools/fury-config/target/release/fury-config")
	f := fury.New(dirs, cfgBin)
	key := make([]byte, 32)
	rand.Read(key)
	lat, lon := 40.71, -74.0
	prof := store.Profile{ID: "spike", Name: "spike", PersonaID: "macos-m4-1710x1112",
		SeedHex: os.Getenv("SPIKE_SEED"), Timezone: "America/New_York", Languages: "en-US,en", UILocale: "en-US",
		Lat: &lat, Lon: &lon}
	udd := dirs.Profile(prof.ID)
	os.MkdirAll(udd, 0o700)
	var samples []string
	for launch := 1; launch <= 2; launch++ {
		s, err := once(f, prof, udd, key, launch)
		if err != nil {
			return fmt.Errorf("launch %d: %w", launch, err)
		}
		samples = append(samples, s)
	}
	fmt.Println("SAME FINGERPRINT ACROSS LAUNCHES:", samples[0] == samples[1])
	return nil
}

func once(f *fury.Fury, prof store.Profile, udd string, key []byte, launch int) (string, error) {
	spec, err := f.BuildLaunch(context.Background(), prof, engine.Runtime{UserDataDir: udd, CookieKey: key})
	if err != nil {
		return "", err
	}
	args := make([]string, 0, len(spec.Args)+1)
	for _, a := range spec.Args {
		switch a {
		case "--fury-fp-fd=3":
			a = "--fury-fp-fd=5"
		case "--fury-oscrypt-fd=4":
			a = "--fury-oscrypt-fd=6"
		}
		args = append(args, a)
	}
	args = append(args, "--remote-debugging-pipe")
	cmdR, cmdW, _ := os.Pipe()   // browser reads commands on fd 3
	respR, respW, _ := os.Pipe() // browser writes replies on fd 4
	cfgR, cfgW, _ := os.Pipe()   // fd 5
	keyR, keyW, _ := os.Pipe()   // fd 6
	cmd := exec.Command(spec.Binary, args...)
	cmd.Env = append(os.Environ(), spec.Env...)
	// fds 7 and 8: the browser also holds the app's ends, so it never sees EOF
	// on its command pipe when the app closes them or crashes.
	cmd.ExtraFiles = []*os.File{cmdR, respW, cfgR, keyR, cmdW, respR}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	logf, _ := os.Create(filepath.Join(udd, "..", fmt.Sprintf("spike-%d.log", launch)))
	cmd.Stdout, cmd.Stderr = logf, logf
	if err := cmd.Start(); err != nil {
		return "", err
	}
	for _, c := range []*os.File{cmdR, respW, cfgR, keyR} {
		c.Close()
	}
	go func() { cfgW.Write(spec.Pipes[0]); cfgW.Close() }()
	go func() { keyW.Write(key); keyW.Close() }()
	pid := cmd.Process.Pid
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	defer func() {
		syscall.Kill(pid, syscall.SIGHUP)
		select {
		case <-exited:
		case <-time.After(8 * time.Second):
			syscall.Kill(-pid, syscall.SIGKILL)
		}
	}()

	c := newClient(cmdW, respR)
	v, err := c.call("", "Browser.getVersion", nil)
	if err != nil {
		return "", fmt.Errorf("CDP over pipe: %w", err)
	}
	fmt.Printf("[%d] 1-2 fds 5/6 + pipe OK: %s\n", launch, v)

	t0 := time.Now()
	r, err := c.call("", "Target.createTarget", map[string]any{"url": "about:blank", "background": true, "hidden": true})
	if err != nil {
		return "", err
	}
	var tgt struct{ TargetID string }
	json.Unmarshal(r, &tgt)
	r, err = c.call("", "Target.attachToTarget", map[string]any{"targetId": tgt.TargetID, "flatten": true})
	if err != nil {
		return "", err
	}
	var att struct{ SessionID string }
	json.Unmarshal(r, &att)
	sid := att.SessionID
	if _, err := c.call(sid, "Fetch.enable", map[string]any{"patterns": []map[string]string{{"urlPattern": probeOrigin + "*"}}}); err != nil {
		return "", err
	}
	go func() {
		for ev := range c.events {
			if ev.Method == "Fetch.requestPaused" {
				var p struct{ RequestID string }
				json.Unmarshal(ev.Params, &p)
				body := base64.StdEncoding.EncodeToString([]byte("<!doctype html><title>probe</title>"))
				c.call(ev.SessionID, "Fetch.fulfillRequest", map[string]any{"requestId": p.RequestID, "responseCode": 200,
					"responseHeaders": []map[string]string{{"name": "Content-Type", "value": "text/html"}}, "body": body})
			}
		}
	}()
	if _, err := c.call(sid, "Page.navigate", map[string]any{"url": probeOrigin}); err != nil {
		return "", err
	}
	var out struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails json.RawMessage `json:"exceptionDetails"`
	}
	for i := 0; i < 50; i++ {
		r, err = c.call(sid, "Runtime.evaluate", map[string]any{"expression": "location.href + '|' + document.readyState", "returnByValue": true})
		if err == nil && strings.Contains(string(r), probeOrigin+"|complete") {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	r, err = c.call(sid, "Runtime.evaluate", map[string]any{"expression": probeJS, "awaitPromise": true, "returnByValue": true})
	if err != nil {
		return "", err
	}
	json.Unmarshal(r, &out)
	if len(out.ExceptionDetails) > 0 {
		return "", fmt.Errorf("probe threw: %s", out.ExceptionDetails)
	}
	c.call("", "Target.closeTarget", map[string]any{"targetId": tgt.TargetID})
	fmt.Printf("[%d] 3 probe in %v: %s\n", launch, time.Since(t0).Round(time.Millisecond), out.Result.Value)

	cmdW.Close()
	respR.Close()
	time.Sleep(5 * time.Second)
	alive := syscall.Kill(pid, 0) == nil
	lsof, _ := exec.Command("lsof", "-a", "-nP", "-iTCP", "-sTCP:LISTEN", "-p", fmt.Sprint(pid)).Output()
	fmt.Printf("[%d] 4 alive after pipe closed: %v, TCP listeners: %d\n", launch, alive, strings.Count(string(lsof), "\n"))
	if !alive {
		return string(out.Result.Value), errors.New("browser exited when the pipe closed")
	}
	return string(out.Result.Value), nil
}
