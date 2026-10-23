// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package vm

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

// adbServerPort is AnyProfile's own adb server, apart from the user's (5037):
// restarting one never drops the other's devices.
const adbServerPort = 5137

// adb runs adb commands against one device at 127.0.0.1:port.
type adb struct {
	bin  string
	port int
}

func (a adb) serial() string { return "127.0.0.1:" + strconv.Itoa(a.port) }

func (a adb) env() []string {
	return append(os.Environ(), "ANDROID_ADB_SERVER_PORT="+strconv.Itoa(adbServerPort))
}

// run is adb -s <device> args...; the output is trimmed.
func (a adb) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, a.bin, append([]string{"-s", a.serial()}, args...)...)
	cmd.Env = a.env()
	out, err := cmd.CombinedOutput()
	s := strings.TrimSpace(string(bytes.ReplaceAll(out, []byte("\r"), nil)))
	if err != nil {
		return s, fmt.Errorf("adb %s: %v: %s", strings.Join(args, " "), err, s)
	}
	return s, nil
}

// shell runs one shell command line on the device.
func (a adb) shell(ctx context.Context, line string) (string, error) {
	return a.run(ctx, "shell", line)
}

func (a adb) connect(ctx context.Context) {
	cmd := exec.CommandContext(ctx, a.bin, "connect", a.serial())
	cmd.Env = a.env()
	_ = cmd.Run()
}

// disconnect forgets the device in the adb server (its port is not reused soon,
// but a stale entry shows as offline in adb devices).
func (a adb) disconnect() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, a.bin, "disconnect", a.serial())
	cmd.Env = a.env()
	_ = cmd.Run()
}

// waitBoot waits until Android reports boot_completed, or exited closes.
func (a adb) waitBoot(ctx context.Context, exited <-chan struct{}) error {
	for {
		select {
		case <-ctx.Done():
			return errors.New("Android did not finish starting in time")
		case <-exited:
			return errors.New("the virtual machine stopped while starting")
		default:
		}
		a.connect(ctx)
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		out, err := a.shell(c, "getprop sys.boot_completed")
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
