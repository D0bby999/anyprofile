// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import "unsafe"

// frontPID is 0: Windows does not let a background process take the
// foreground, so a browser started from the API stays behind on its own.
func frontPID() int { return 0 }

func setHidden(int, bool) bool { return false }

func activateSelf() {}

func PassFocus() int { return 0 }

// workArea is the primary screen without the taskbar, in DIPs: the app is
// DPI aware, so Windows answers in pixels.
func workArea() (Rect, bool) {
	var r struct{ Left, Top, Right, Bottom int32 }
	if ok, _, _ := procSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&r)), 0); ok == 0 {
		return Rect{}, false
	}
	dpi, _, _ := procGetDpiForSystem.Call()
	if dpi == 0 {
		dpi = 96
	}
	k := func(v int32) int { return int(v) * 96 / int(dpi) }
	return Rect{X: k(r.Left), Y: k(r.Top), W: k(r.Right - r.Left), H: k(r.Bottom - r.Top)}, r.Right > r.Left
}

// Windows has no desktop that would jump: every browser is brought up.
func onCurrentDesktop(int) bool { return true }

// activate brings the browser's first window to the front, restoring it when
// minimized. Windows lets the app do this because it is the foreground
// process when the user clicks Open.
func activate(pid int) bool {
	wins := browserWindows(pid)
	if len(wins) == 0 {
		return false
	}
	if min, _, _ := procIsIconic.Call(wins[0]); min != 0 {
		procShowWindow.Call(wins[0], swRestore)
	}
	ok, _, _ := procSetForegroundWindow.Call(wins[0])
	return ok != 0
}
