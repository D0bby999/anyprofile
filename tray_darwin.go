//go:build darwin

// SPDX-License-Identifier: AGPL-3.0-or-later

package main

/*
#cgo darwin CFLAGS: -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa
#include <stdlib.h>

void anyprofileTrayInit(
	int active,
	int hasRunning,
	const char *statusLabel,
	const char *openLabel,
	const char *settingsLabel,
	const char *updatesLabel,
	const char *stopAllLabel,
	const char *quitLabel,
	const char *tooltip);

void anyprofileTrayUpdate(
	int active,
	int hasRunning,
	const char *statusLabel,
	const char *openLabel,
	const char *settingsLabel,
	const char *updatesLabel,
	const char *stopAllLabel,
	const char *quitLabel,
	const char *tooltip);

void anyprofileTrayDestroy(void);
void anyprofileTrayWakeRunLoop(void);
*/
import "C"

import (
	"sync"
	"unsafe"
)

const (
	trayActionOpen     = 1
	trayActionSettings = 2
	trayActionUpdates  = 3
	trayActionStopAll  = 4
	trayActionQuit     = 5
)

var (
	trayMu  sync.RWMutex
	trayApp *App
)

func boolToCInt(v bool) C.int {
	if v {
		return 1
	}
	return 0
}

func withTrayStrings(ts trayState, fn func(cStatus, cOpen, cSettings, cUpdates, cStopAll, cQuit, cTip *C.char)) {
	cStatus := C.CString(ts.StatusLabel)
	cOpen := C.CString(ts.OpenLabel)
	cSettings := C.CString(ts.SettingsLabel)
	cUpdates := C.CString(ts.UpdatesLabel)
	cStopAll := C.CString(ts.StopAllLabel)
	cQuit := C.CString(ts.QuitLabel)
	cTip := C.CString(ts.Tooltip)
	defer C.free(unsafe.Pointer(cStatus))
	defer C.free(unsafe.Pointer(cOpen))
	defer C.free(unsafe.Pointer(cSettings))
	defer C.free(unsafe.Pointer(cUpdates))
	defer C.free(unsafe.Pointer(cStopAll))
	defer C.free(unsafe.Pointer(cQuit))
	defer C.free(unsafe.Pointer(cTip))
	fn(cStatus, cOpen, cSettings, cUpdates, cStopAll, cQuit, cTip)
}

func initTray(app *App, ts trayState) {
	trayMu.Lock()
	trayApp = app
	trayMu.Unlock()

	withTrayStrings(ts, func(cStatus, cOpen, cSettings, cUpdates, cStopAll, cQuit, cTip *C.char) {
		C.anyprofileTrayInit(
			boolToCInt(ts.Active),
			boolToCInt(ts.HasRunning),
			cStatus, cOpen, cSettings, cUpdates, cStopAll, cQuit, cTip,
		)
	})
}

func updateTrayState(ts trayState) {
	trayMu.RLock()
	activeApp := trayApp
	trayMu.RUnlock()
	if activeApp == nil {
		return
	}

	withTrayStrings(ts, func(cStatus, cOpen, cSettings, cUpdates, cStopAll, cQuit, cTip *C.char) {
		C.anyprofileTrayUpdate(
			boolToCInt(ts.Active),
			boolToCInt(ts.HasRunning),
			cStatus, cOpen, cSettings, cUpdates, cStopAll, cQuit, cTip,
		)
	})
}

func stopTray() {
	trayMu.Lock()
	wasInit := trayApp != nil
	trayApp = nil
	trayMu.Unlock()
	if wasInit {
		C.anyprofileTrayDestroy()
	}
}

func wakeTrayRunLoop() {
	C.anyprofileTrayWakeRunLoop()
}

//export anyprofileTrayMenuAction
func anyprofileTrayMenuAction(action C.int) {
	trayMu.RLock()
	app := trayApp
	trayMu.RUnlock()
	if app == nil {
		return
	}

	switch int(action) {
	case trayActionOpen:
		go app.showMainWindow()
	case trayActionSettings:
		go app.openMenuDestination("settings")
	case trayActionUpdates:
		go app.openMenuDestination("check-update")
	case trayActionStopAll:
		go app.stopAllFromTray()
	case trayActionQuit:
		go app.requestQuit()
	}
}
