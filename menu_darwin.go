// SPDX-License-Identifier: AGPL-3.0-or-later

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>

static void aboutPanel(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApp orderFrontStandardAboutPanel:nil];
		[NSApp activateIgnoringOtherApps:YES];
	});
}
*/
import "C"

import (
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var menuText = map[string]struct{ About, Updates, Settings, Hide, Quit string }{
	"en": {"About AnyProfile", "Check for Updates…", "Settings…", "Hide AnyProfile", "Quit AnyProfile"},
	"vi": {"Giới thiệu AnyProfile", "Kiểm tra bản cập nhật…", "Cài đặt…", "Ẩn AnyProfile", "Thoát AnyProfile"},
}

// appMenu is the macOS menu bar: the app menu with Check for Updates and
// Settings, as Mac apps have them, then Edit (copy and paste) and Window.
// The UI hears the choices as "menu" events.
func appMenu(a *App, lang string) *menu.Menu {
	tx, ok := menuText[lang]
	if !ok {
		tx = menuText["en"]
	}
	emit := func(what string) func(*menu.CallbackData) {
		return func(*menu.CallbackData) {
			runtime.WindowShow(a.ctx)
			runtime.EventsEmit(a.ctx, "menu", what)
		}
	}
	m := menu.NewMenu()
	app := m.AddSubmenu("AnyProfile")
	app.AddText(tx.About, nil, func(*menu.CallbackData) { C.aboutPanel() })
	app.AddSeparator()
	app.AddText(tx.Updates, nil, emit("check-update"))
	app.AddText(tx.Settings, keys.CmdOrCtrl(","), emit("settings"))
	app.AddSeparator()
	app.AddText(tx.Hide, keys.CmdOrCtrl("h"), func(*menu.CallbackData) { runtime.Hide(a.ctx) })
	app.AddText(tx.Quit, keys.CmdOrCtrl("q"), func(*menu.CallbackData) { runtime.Quit(a.ctx) })
	m.Append(menu.EditMenu())
	m.Append(menu.WindowMenu())
	return m
}

// setMenuLanguage rebuilds the menu bar in the app's language.
func (a *App) setMenuLanguage(lang string) {
	runtime.MenuSetApplicationMenu(a.ctx, appMenu(a, lang))
	runtime.MenuUpdateApplicationMenu(a.ctx)
}
