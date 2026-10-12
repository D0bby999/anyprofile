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
*/
import "C"

// activate brings the browser's windows to the front. NSRunningApplication
// needs no Accessibility permission.
func activate(pid int) bool { return C.activatePID(C.int(pid)) == 1 }
