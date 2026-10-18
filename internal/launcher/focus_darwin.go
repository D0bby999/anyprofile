// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit -framework CoreGraphics
#import <AppKit/AppKit.h>

static int activatePID(int pid) {
	@autoreleasepool {
		NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
		if (app == nil) return 0;
		return [app activateWithOptions:NSApplicationActivateAllWindows] ? 1 : 0;
	}
}

// activateSelf puts this app (AnyProfile) back in front. Unlike hiding
// another app, it never lets macOS pick an app on another desktop.
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
static void activateSelf(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApp activateIgnoringOtherApps:YES];
	});
}
#pragma clang diagnostic pop

// passFocus activates the app owning the frontmost window on the current
// desktop, other than this app; 0 when there is none.
static int passFocus(void) {
	@autoreleasepool {
		pid_t self = getpid();
		if ([[NSWorkspace sharedWorkspace] frontmostApplication].processIdentifier != self) {
			return 0; // quit from the Dock while in another app: leave it be
		}
		CFArrayRef list = CGWindowListCopyWindowInfo(
			kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
		if (list == NULL) return 0;
		NSArray *wins = CFBridgingRelease(list);
		for (NSDictionary *w in wins) {
			if ([w[(id)kCGWindowLayer] intValue] != 0) continue;
			if ([w[(id)kCGWindowAlpha] doubleValue] <= 0) continue;
			pid_t pid = [w[(id)kCGWindowOwnerPID] intValue];
			if (pid == self) continue;
			NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
			if (app == nil || app.activationPolicy != NSApplicationActivationPolicyRegular) continue;
			return [app activateWithOptions:0] ? pid : 0;
		}
		return 0;
	}
}

// workArea is the visible frame (no menu bar, no Dock) of the screen with
// the key window, in points from the top left of the main screen.
static int workArea(int *x, int *y, int *w, int *h) {
	__block int ok = 0;
	void (^read)(void) = ^{
		NSScreen *s = [NSScreen mainScreen];
		NSArray *all = [NSScreen screens];
		if (s == nil || all.count == 0) return;
		NSRect v = s.visibleFrame;
		CGFloat top = ((NSScreen *)all[0]).frame.size.height;
		*x = (int)v.origin.x;
		*y = (int)(top - (v.origin.y + v.size.height));
		*w = (int)v.size.width;
		*h = (int)v.size.height;
		ok = 1;
	};
	if ([NSThread isMainThread]) read(); else dispatch_sync(dispatch_get_main_queue(), read);
	return ok;
}

// onScreen: pid has a normal window on the current desktop.
static int onScreen(int pid) {
	@autoreleasepool {
		CFArrayRef list = CGWindowListCopyWindowInfo(
			kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
		if (list == NULL) return 0;
		NSArray *wins = CFBridgingRelease(list);
		for (NSDictionary *w in wins) {
			if ([w[(id)kCGWindowOwnerPID] intValue] == pid && [w[(id)kCGWindowLayer] intValue] == 0) return 1;
		}
		return 0;
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

// activateSelf brings AnyProfile itself back in front.
func activateSelf() { C.activateSelf() }

// PassFocus hands the focus, while AnyProfile still has it, to the app
// owning the frontmost window on the current desktop. Called as the app
// quits: a front app that quits lets macOS pick the next app itself, maybe
// a full-screen one on another desktop, and the screen jumped there.
func PassFocus() int { return int(C.passFocus()) }

func workArea() (Rect, bool) {
	var x, y, w, h C.int
	if C.workArea(&x, &y, &w, &h) == 0 || w <= 0 || h <= 0 {
		return Rect{}, false
	}
	return Rect{X: int(x), Y: int(y), W: int(w), H: int(h)}, true
}

func onCurrentDesktop(pid int) bool { return C.onScreen(C.int(pid)) == 1 }

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
