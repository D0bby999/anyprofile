// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"time"
	"unsafe"

	"github.com/D0bby999/anyprofile/internal/cdppipe"
	"golang.org/x/sys/windows"
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procFindWindowExW            = user32.NewProc("FindWindowExW")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procIsIconic                 = user32.NewProc("IsIconic")
	procShowWindow               = user32.NewProc("ShowWindow")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procPostMessageW             = user32.NewProc("PostMessageW")
	procGetClassNameW            = user32.NewProc("GetClassNameW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
)

const (
	stillActive = 259         // GetExitCodeProcess of a running process
	hwndMessage = ^uintptr(2) // HWND_MESSAGE, (HWND)-3
	wmClose     = 0x0010
	swRestore   = 9
	// Chromium's browser windows; its hidden message windows use other classes.
	browserWindowClass = "Chrome_WidgetWin_1"
)

// pidStartTime is the process creation time, as a string to store. It
// distinguishes a surviving browser from a recycled pid after a reboot.
func pidStartTime(pid int) (string, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil {
		return "", err
	}
	return strconv.FormatInt(created.Nanoseconds(), 10), nil
}

func alive(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return errors.Is(err, windows.ERROR_ACCESS_DENIED)
	}
	defer windows.CloseHandle(h)
	var code uint32
	return windows.GetExitCodeProcess(h, &code) == nil && code == stillActive
}

// singletonPID finds the browser that owns userDataDir: Chromium on Windows
// keeps a message-only window of class Chrome_MessageWindow titled with the
// user data dir (process_singleton_win.cc). 0 when there is none.
func singletonPID(userDataDir string) int {
	cls, _ := windows.UTF16PtrFromString("Chrome_MessageWindow")
	title, err := windows.UTF16PtrFromString(filepath.Clean(userDataDir))
	if err != nil {
		return 0
	}
	hwnd, _, _ := procFindWindowExW.Call(hwndMessage, 0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(title)))
	if hwnd == 0 {
		return 0
	}
	return windowPID(hwnd)
}

// sameProcess is true when pid is alive and is the process recorded earlier.
func sameProcess(pid int, startTime string) bool {
	if !alive(pid) {
		return false
	}
	st, err := pidStartTime(pid)
	return err == nil && st == startTime
}

// askQuit asks the browser to quit the way closing it from its menu does,
// saving its session and cookie jar. Windows has no SIGHUP: with a DevTools
// pipe the browser gets Browser.close, otherwise (adopted after the app
// restarted) each of its windows gets WM_CLOSE.
func askQuit(pid int, cdp *cdppipe.Conn) {
	if cdp != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if _, err := cdp.Call(ctx, "", "Browser.close", nil); err == nil {
			return
		}
	}
	for _, hwnd := range browserWindows(pid) {
		procPostMessageW.Call(hwnd, wmClose, 0, 0)
	}
}

// killTree ends the browser and every process it started; unsaved data is lost.
func killTree(pid int) {
	for _, id := range processTree(pid) {
		terminate(id)
	}
}

// processTree is pid followed by every process descended from it.
func processTree(pid int) []uint32 {
	tree := []uint32{uint32(pid)}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return tree
	}
	defer windows.CloseHandle(snap)
	children := map[uint32][]uint32{}
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err := windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		children[e.ParentProcessID] = append(children[e.ParentProcessID], e.ProcessID)
	}
	for i := 0; i < len(tree); i++ {
		tree = append(tree, children[tree[i]]...)
	}
	return tree
}

func terminate(pid uint32) {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return
	}
	windows.TerminateProcess(h, 1)
	windows.CloseHandle(h)
}

func windowPID(hwnd uintptr) int {
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return int(pid)
}

// browserWindows lists pid's visible top-level browser windows.
func browserWindows(pid int) []uintptr {
	var out []uintptr
	cb := windows.NewCallback(func(hwnd, _ uintptr) uintptr {
		if windowPID(hwnd) != pid {
			return 1
		}
		if v, _, _ := procIsWindowVisible.Call(hwnd); v == 0 {
			return 1
		}
		buf := make([]uint16, 64)
		n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if windows.UTF16ToString(buf[:n]) == browserWindowClass {
			out = append(out, hwnd)
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
	return out
}
