// SPDX-License-Identifier: AGPL-3.0-or-later

// Package site is the catalog of platforms a profile can be for (Etsy,
// TikTok, ...). It is unrelated to store.Profile.Platform, the persona's OS.
package site

import (
	_ "embed"
	"encoding/json"
	"sync"
)

// Other is the catch-all site, and the default.
const Other = "other"

type Site struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Group    string   `json:"group"`
	Domains  []string `json:"domains"`
	StartURL string   `json:"startUrl"`
	Icon     string   `json:"icon"`  // simple-icons slug, "" for a letter badge
	Color    string   `json:"color"` // "#RRGGBB"
	Custom   bool     `json:"custom"`
}

//go:embed catalog.json
var catalogJSON []byte

var (
	once    sync.Once
	builtin []Site
)

// Builtin returns a copy of the shipped catalog.
func Builtin() []Site {
	once.Do(func() {
		if err := json.Unmarshal(catalogJSON, &builtin); err != nil {
			panic("site catalog: " + err.Error()) // caught by TestCatalog
		}
	})
	return append([]Site(nil), builtin...)
}
