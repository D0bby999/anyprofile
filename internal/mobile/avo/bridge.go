// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package avo

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// adbBridge listens on an ephemeral 127.0.0.1 TCP port and tunnels each ADB
// connection over anyprofile-vmm's virtio-vsock Unix socket (CONNECT 5555\n).
type adbBridge struct {
	Port      int
	ln        net.Listener
	vsockPath string
	done      chan struct{}
	mu        sync.Mutex
	conns     map[net.Conn]struct{}
}

func startADBBridge(vsockPath string) (*adbBridge, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	b := &adbBridge{
		Port:      port,
		ln:        ln,
		vsockPath: vsockPath,
		done:      make(chan struct{}),
		conns:     make(map[net.Conn]struct{}),
	}
	go b.serve()
	return b, nil
}

func (b *adbBridge) Close() error {
	select {
	case <-b.done:
		return nil
	default:
		close(b.done)
	}
	err := b.ln.Close()
	b.mu.Lock()
	for c := range b.conns {
		_ = c.Close()
	}
	b.mu.Unlock()
	return err
}

func (b *adbBridge) track(c net.Conn) {
	b.mu.Lock()
	b.conns[c] = struct{}{}
	b.mu.Unlock()
}

func (b *adbBridge) untrack(c net.Conn) {
	b.mu.Lock()
	delete(b.conns, c)
	b.mu.Unlock()
}

func (b *adbBridge) serve() {
	for {
		c, err := b.ln.Accept()
		if err != nil {
			return
		}
		go b.handle(c)
	}
}

func (b *adbBridge) handle(client net.Conn) {
	defer client.Close()
	select {
	case <-b.done:
		return
	default:
	}
	vs, err := net.DialTimeout("unix", b.vsockPath, 5*time.Second)
	if err != nil {
		return
	}
	defer vs.Close()

	_ = vs.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.WriteString(vs, "CONNECT 5555\n"); err != nil {
		return
	}
	br := bufio.NewReader(vs)
	reply, err := br.ReadString('\n')
	if err != nil || !strings.HasPrefix(reply, "OK ") {
		return
	}
	_ = vs.SetDeadline(time.Time{})

	b.track(client)
	b.track(vs)
	defer func() {
		b.untrack(client)
		b.untrack(vs)
	}()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(vs, client)
		_ = vs.Close()
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(client, br)
		_ = client.Close()
	}()
	wg.Wait()
}

// sendControl sends a single command line over avo-vmm's control.sock and
// checks for an "ok" reply.
func sendControl(ctx context.Context, sockPath, cmd string) (string, error) {
	d := net.Dialer{Timeout: 4 * time.Second}
	c, err := d.DialContext(ctx, "unix", sockPath)
	if err != nil {
		return "", err
	}
	defer c.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = c.SetDeadline(dl)
	} else {
		_ = c.SetDeadline(time.Now().Add(6 * time.Second))
	}
	if _, err := io.WriteString(c, strings.TrimSpace(cmd)+"\n"); err != nil {
		return "", err
	}
	line, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "ok" || strings.HasPrefix(line, "ok ") {
		return line, nil
	}
	if msg, ok := strings.CutPrefix(line, "error "); ok {
		return line, errors.New(msg)
	}
	return line, fmt.Errorf("control %q: %s", cmd, line)
}
