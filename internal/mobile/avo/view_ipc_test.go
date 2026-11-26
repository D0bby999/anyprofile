// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package avo

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile"
)

func TestViewIPCZeroFlickerArrangeAndEvents(t *testing.T) {
	shortDir, err := os.MkdirTemp("/tmp", "ap-vipc-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(shortDir)
	sock := filepath.Join(shortDir, "v.sock")
	gotFocus := make(chan bool, 1)
	connected := make(chan struct{}, 1)

	ipc, err := startViewIPC(sock, func(v *viewIPC) {
		select {
		case connected <- struct{}{}:
		default:
		}
	}, func(ev viewEvent) {
		if ev.Ev == "focus" {
			gotFocus <- true
		}
	})
	if err != nil {
		t.Fatalf("startViewIPC: %v", err)
	}
	defer ipc.close()

	client, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("dial view.sock: %v", err)
	}
	defer client.Close()

	select {
	case <-connected:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for server accept")
	}

	d := New(t.TempDir(), t.TempDir(), nil)
	dev := mobile.Device{ID: "dev1", Name: "Phone 1", ScreenW: 780, ScreenH: 1688}
	// Dummy running view marker; zero-flicker path must send set-frame over IPC without closing it.
	d.procs[dev.ID] = &proc{view: &exec.Cmd{}, ipc: ipc}

	n := d.ArrangeWindows(context.Background(), "row", []mobile.Device{dev})
	if n != 1 {
		t.Fatalf("expected 1 arranged window, got %d", n)
	}

	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	line, err := bufio.NewReader(client).ReadBytes('\n')
	if err != nil {
		t.Fatalf("read set-frame: %v", err)
	}
	var cmd map[string]any
	if err := json.Unmarshal(line, &cmd); err != nil {
		t.Fatalf("unmarshal cmd: %v", err)
	}
	if cmd["cmd"] != "set-frame" {
		t.Fatalf("expected set-frame command, got %v", cmd)
	}

	if _, err := client.Write([]byte(`{"ev":"focus"}` + "\n")); err != nil {
		t.Fatalf("write focus event: %v", err)
	}
	select {
	case <-gotFocus:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for focus event")
	}
}
