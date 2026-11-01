// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package vm

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit -framework CoreGraphics
#import <AppKit/AppKit.h>

// windowBounds reads the visible frame of the first normal window of pid, in
// screen points from the top left. CGWindowListCopyWindowInfo needs no
// Accessibility or Screen Recording permission for bounds.
static int windowBounds(int pid, int *x, int *y, int *w, int *h) {
	@autoreleasepool {
		CFArrayRef list = CGWindowListCopyWindowInfo(
			kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
		if (list == NULL) return 0;
		NSArray *wins = CFBridgingRelease(list);
		for (NSDictionary *win in wins) {
			if ([win[(id)kCGWindowOwnerPID] intValue] != pid) continue;
			if ([win[(id)kCGWindowLayer] intValue] != 0) continue;
			NSDictionary *b = win[(id)kCGWindowBounds];
			if (b == nil) continue;
			*x = [b[@"X"] intValue];
			*y = [b[@"Y"] intValue];
			*w = [b[@"Width"] intValue];
			*h = [b[@"Height"] intValue];
			return *w > 0 && *h > 0;
		}
		return 0;
	}
}

// workArea is the visible frame (no menu bar, no Dock) of the main screen.
static int workArea(int *x, int *y, int *w, int *h) {
	__block int ok = 0;
	void (^read)(void) = ^{
		NSScreen *s = [NSScreen mainScreen];
		if (s == nil) return;
		NSRect v = s.visibleFrame;
		CGFloat top = s.frame.size.height;
		*x = (int)v.origin.x;
		*y = (int)(top - (v.origin.y + v.size.height));
		*w = (int)v.size.width;
		*h = (int)v.size.height;
		ok = *w > 0 && *h > 0;
	};
	if ([NSThread isMainThread]) read(); else dispatch_sync(dispatch_get_main_queue(), read);
	return ok;
}
*/
import "C"

type windowRect struct{ X, Y, W, H int }

func scrcpyWindow(pid int) (windowRect, bool) {
	var x, y, w, h C.int
	if C.windowBounds(C.int(pid), &x, &y, &w, &h) == 0 {
		return windowRect{}, false
	}
	return windowRect{X: int(x), Y: int(y), W: int(w), H: int(h)}, true
}

func screenWorkArea() (windowRect, bool) {
	var x, y, w, h C.int
	if C.workArea(&x, &y, &w, &h) == 0 {
		return windowRect{}, false
	}
	return windowRect{X: int(x), Y: int(y), W: int(w), H: int(h)}, true
}
