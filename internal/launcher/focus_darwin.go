// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>

static int activatePID(int pid) {
	@autoreleasepool {
		NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
		if (app == nil) return 0;
		return [app activateWithOptions:NSApplicationActivateAllWindows] ? 1 : 0;
	}
}

static int frontPID(void) {
	@autoreleasepool {
		NSRunningApplication *app = [[NSWorkspace sharedWorkspace] frontmostApplication];
		return app == nil ? 0 : app.processIdentifier;
	}
}

static int setHidden(int pid, int hide) {
	@autoreleasepool {
		NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
		if (app == nil) return 0;
		return (hide ? [app hide] : [app unhide]) ? 1 : 0;
	}
}
*/
import "C"

// activate brings the browser's windows to the front. NSRunningApplication
// needs no Accessibility permission.
func activate(pid int) bool { return C.activatePID(C.int(pid)) == 1 }

// frontPID is the process of the app in front, 0 when unknown.
func frontPID() int { return int(C.frontPID()) }

// setHidden hides or shows all of pid's windows, like Cmd-H.
func setHidden(pid int, hide bool) bool {
	h := 0
	if hide {
		h = 1
	}
	return C.setHidden(C.int(pid), C.int(h)) == 1
}
