// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package avo

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>

static int activateViewPID(int pid) {
	@autoreleasepool {
		NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
		if (app == nil) return 0;
		return [app activateWithOptions:NSApplicationActivateAllWindows] ? 1 : 0;
	}
}
*/
import "C"

var activateView = func(pid int) bool {
	return C.activateViewPID(C.int(pid)) != 0
}
