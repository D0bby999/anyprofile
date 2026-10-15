// SPDX-License-Identifier: AGPL-3.0-or-later

// Package site is the catalog of platforms a profile can be for (Etsy,
// TikTok, ...). It is unrelated to store.Profile.Platform, the persona's OS.
package site

import (
	_ "embed"
	"encoding/json"
	"strings"
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
	// StartURLs are the start pages of a platform that differs by country
	// (TikTok Shop's seller centers), keyed by ISO country code. StartURL is
	// the one for any other country, or when the country is unknown.
	StartURLs map[string]string `json:"startUrls,omitempty"`
	Icon      string            `json:"icon"`  // simple-icons slug, "" for a letter badge
	Color     string            `json:"color"` // "#RRGGBB"
	Custom    bool              `json:"custom"`
}

// StartURLFor is the start page for a profile whose proxy exits in country cc.
func (s Site) StartURLFor(cc string) string {
	if u, ok := s.StartURLs[strings.ToUpper(cc)]; ok {
		return u
	}
	return s.StartURL
}

// IsStartURL reports whether u is one of the platform's start pages, for any country.
func (s Site) IsStartURL(u string) bool {
	if u == "" {
		return false
	}
	if u == s.StartURL {
		return true
	}
	for _, v := range s.StartURLs {
		if u == v {
			return true
		}
	}
	return false
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
