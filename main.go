// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"embed"
	"log"
	goruntime "runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	ownerOnlyFiles()
	app := NewApp()
	err := wails.Run(&options.App{
		Title:     "AnyProfile",
		Width:     1200,
		Height:    760,
		MinWidth:  960,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Menu:              appMenu(app, "en"),
		HideWindowOnClose: goruntime.GOOS == "darwin",
		OnStartup:         app.startup,
		OnBeforeClose:     app.beforeClose,
		OnShutdown:        app.shutdown,
		// Two instances would each try to manage the same browsers.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "dev.anyprofile.app",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				app.showMainWindow()
			},
		},
		// The sidebar runs under the traffic lights, like a native Mac app.
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About:    &mac.AboutInfo{Title: "AnyProfile", Message: "Multi-profile browser manager for creators.\nAGPL-3.0-or-later."},
		},
		Bind: []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
