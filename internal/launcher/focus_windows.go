// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

// frontPID is 0: Windows does not let a background process take the
// foreground, so a browser started from the API stays behind on its own.
func frontPID() int { return 0 }

func setHidden(int, bool) bool { return false }

func activateSelf() {}

func PassFocus() int { return 0 }

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
