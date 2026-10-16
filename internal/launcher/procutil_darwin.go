// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/D0bby999/anyprofile/internal/cdppipe"
	"golang.org/x/sys/unix"
)

// pidStartTime is the kernel's start time of pid, as a string to store. It
// distinguishes a surviving browser from a recycled pid after a reboot.
func pidStartTime(pid int) (string, error) {
	kp, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return "", err
	}
	if kp.Proc.P_pid != int32(pid) {
		return "", fmt.Errorf("pid %d not found", pid)
	}
	tv := kp.Proc.P_starttime
	return strconv.FormatInt(tv.Sec, 10) + "." + strconv.FormatInt(int64(tv.Usec), 10), nil
}

func alive(pid int) bool {
	err := unix.Kill(pid, 0)
	return err == nil || errors.Is(err, unix.EPERM)
}

// singletonPID reads Chromium's <user-data-dir>/SingletonLock, a symlink to
// "<hostname>-<pid>". 0 when absent or unreadable.
func singletonPID(userDataDir string) int {
	target, err := os.Readlink(filepath.Join(userDataDir, "SingletonLock"))
	if err != nil {
		return 0
	}
	i := strings.LastIndexByte(target, '-')
	if i < 0 {
		return 0
	}
	pid, _ := strconv.Atoi(target[i+1:])
	return pid
}

// sameProcess is true when pid is alive and is the process recorded earlier.
func sameProcess(pid int, startTime string) bool {
	if !alive(pid) {
		return false
	}
	st, err := pidStartTime(pid)
	return err == nil && st == startTime
}

// askQuit asks the browser to quit the way a menu Quit does, saving its
// session and cookie jar. SIGHUP, not SIGTERM: from Chromium 155 a SIGTERM on
// macOS means "the login session is ending" and exits without writing the
// cookie jar (measured by the Fury authors and again here). Main process
// only; it shuts its helpers down.
func askQuit(pid int, _ *cdppipe.Conn) { unix.Kill(pid, unix.SIGHUP) }

// killTree ends the browser and its helpers at once; unsaved data is lost.
func killTree(pid int) { signalGroup(pid, unix.SIGKILL) }

func signalGroup(pid int, sig syscall.Signal) {
	// The browser leads its own process group (Setpgid), so -pid reaches its helpers.
	if err := unix.Kill(-pid, sig); err != nil {
		unix.Kill(pid, sig)
	}
}
