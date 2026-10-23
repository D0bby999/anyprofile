// SPDX-License-Identifier: AGPL-3.0-or-later

// Package adb runs adb against one Android device over TCP, through
// AnyProfile's own adb server: both mobile drivers use it.
package adb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/engine"
)

// ServerPort is AnyProfile's own adb server, apart from the user's (5037)
// and LDPlayer's: restarting one never drops the other's devices.
const ServerPort = 5137

// Client is adb for the device at 127.0.0.1:Port.
type Client struct {
	Bin  string
	Port int
}

func (a Client) Serial() string { return "127.0.0.1:" + strconv.Itoa(a.Port) }

// Env is the environment adb and the tools that call it (scrcpy) run with.
func (a Client) Env() []string {
	return append(os.Environ(), "ANDROID_ADB_SERVER_PORT="+strconv.Itoa(ServerPort))
}

func (a Client) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, a.Bin, args...)
	cmd.Env = a.Env()
	cmd.SysProcAttr = hidden()
	return cmd
}

// Run is adb -s <device> args...; the output is trimmed.
func (a Client) Run(ctx context.Context, args ...string) (string, error) {
	out, err := a.command(ctx, append([]string{"-s", a.Serial()}, args...)...).CombinedOutput()
	s := strings.TrimSpace(string(bytes.ReplaceAll(out, []byte("\r"), nil)))
	if err != nil {
		return s, fmt.Errorf("adb %s: %v: %s", strings.Join(args, " "), err, s)
	}
	return s, nil
}

// Output is adb -s <device> args... with the output as it came (a
// screenshot's bytes); errors carry what adb printed.
func (a Client) Output(ctx context.Context, args ...string) ([]byte, error) {
	cmd := a.command(ctx, append([]string{"-s", a.Serial()}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("adb %s: %v: %s", strings.Join(args, " "), err, bytes.TrimSpace(stderr.Bytes()))
	}
	return out, nil
}

// FetchAPK downloads an app once into dir and checks it by hash.
func FetchAPK(ctx context.Context, dir, name, url, sha string) (string, error) {
	p := filepath.Join(dir, name)
	if engine.VerifySHA256(p, sha) == nil {
		return p, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	tmp := p + ".part"
	if err := engine.Download(ctx, url, tmp, nil); err != nil {
		return "", err
	}
	if err := engine.VerifySHA256(tmp, sha); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return p, os.Rename(tmp, p)
}

// Shell runs one shell command line on the device.
func (a Client) Shell(ctx context.Context, line string) (string, error) {
	return a.Run(ctx, "shell", line)
}

func (a Client) Connect(ctx context.Context) { _ = a.command(ctx, "connect", a.Serial()).Run() }

// Disconnect forgets the device in the adb server.
func (a Client) Disconnect() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = a.command(ctx, "disconnect", a.Serial()).Run()
}

// WaitBoot waits until Android reports boot_completed, or exited closes
// (nil: never).
func (a Client) WaitBoot(ctx context.Context, exited <-chan struct{}) error {
	for {
		select {
		case <-ctx.Done():
			return errors.New("Android did not finish starting in time")
		case <-exited:
			return errors.New("the device stopped while starting")
		default:
		}
		a.Connect(ctx)
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		out, err := a.Shell(c, "getprop sys.boot_completed")
		cancel()
		if err == nil && out == "1" {
			return nil
		}
		select {
		case <-ctx.Done():
		case <-exited:
		case <-time.After(2 * time.Second):
		}
	}
}
