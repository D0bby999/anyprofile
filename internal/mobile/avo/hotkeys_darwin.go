// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package avo

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Carbon -framework CoreFoundation
#include <Carbon/Carbon.h>
#include <dispatch/dispatch.h>

extern void goGlobalHotkeyCallback(int id);

static EventHandlerRef gHotKeyHandler = NULL;
static EventHotKeyRef gHotKeys[3] = {NULL, NULL, NULL};

static OSStatus hotKeyHandler(EventHandlerCallRef next, EventRef evt, void *userData) {
	EventHotKeyID hkID;
	if (GetEventParameter(evt, kEventParamDirectObject, typeEventHotKeyID, NULL, sizeof(hkID), NULL, &hkID) == noErr) {
		goGlobalHotkeyCallback((int)hkID.id);
	}
	return noErr;
}

static void registerHotKeysOnMain(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (gHotKeyHandler == NULL) {
			EventTypeSpec spec = { kEventClassKeyboard, kEventHotKeyPressed };
			InstallApplicationEventHandler(&hotKeyHandler, 1, &spec, NULL, &gHotKeyHandler);
		}
		const UInt32 mods = cmdKey | shiftKey;
		const UInt32 codes[3] = { kVK_ANSI_A, kVK_ANSI_M, kVK_ANSI_B };
		for (int i = 0; i < 3; i++) {
			if (gHotKeys[i] != NULL) continue;
			EventHotKeyID hkID = { 'APMB', (UInt32)(i + 1) };
			RegisterEventHotKey(codes[i], mods, hkID, GetApplicationEventTarget(), 0, &gHotKeys[i]);
		}
	});
}

static void unregisterHotKeysOnMain(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		for (int i = 0; i < 3; i++) {
			if (gHotKeys[i] != NULL) {
				UnregisterEventHotKey(gHotKeys[i]);
				gHotKeys[i] = NULL;
			}
		}
	});
}
*/
import "C"

import "sync"

var (
	hotkeyMu sync.Mutex
	hotkeyCB func(action string)
)

//export goGlobalHotkeyCallback
func goGlobalHotkeyCallback(id C.int) {
	hotkeyMu.Lock()
	cb := hotkeyCB
	hotkeyMu.Unlock()
	if cb == nil {
		return
	}
	var action string
	switch int(id) {
	case 1:
		action = "arrange"
	case 2:
		action = "mute-all"
	case 3:
		action = "boss-hide"
	default:
		return
	}
	go cb(action)
}

// RegisterGlobalHotkeys binds macOS Carbon hotkeys (Cmd+Shift+A/M/B) without
// requiring Accessibility permissions.
func (d *Driver) RegisterGlobalHotkeys(cb func(action string)) {
	hotkeyMu.Lock()
	hotkeyCB = cb
	hotkeyMu.Unlock()
	C.registerHotKeysOnMain()
}

// UnregisterGlobalHotkeys releases the global hotkeys.
func (d *Driver) UnregisterGlobalHotkeys() {
	hotkeyMu.Lock()
	hotkeyCB = nil
	hotkeyMu.Unlock()
	C.unregisterHotKeysOnMain()
}
