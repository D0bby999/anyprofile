// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"
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
		Menu:          appMenu(app, "en"),
		OnStartup:     app.startup,
		OnBeforeClose: app.beforeClose,
		// Two instances would each try to manage the same browsers.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "dev.anyprofile.app",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				runtime.WindowUnminimise(app.ctx)
				runtime.WindowShow(app.ctx)
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
