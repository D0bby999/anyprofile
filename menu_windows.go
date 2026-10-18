// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import "github.com/wailsapp/wails/v2/pkg/menu"

// Windows apps of this kind have no menu bar; Settings holds the update check.
func appMenu(*App, string) *menu.Menu { return nil }

func (a *App) setMenuLanguage(string) {}
