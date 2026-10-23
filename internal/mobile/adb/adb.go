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
	"strconv"
	"strings"
	"time"
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
