// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/D0bby999/anyprofile/internal/engine"
)

func fixture() engine.Release {
	return engine.Release{Engine: "anybrowser", Version: "0.5.1", Chromium: "155.0.8059.31",
		Platform: "windows-amd64", URL: "https://example.com/upstream.zip",
		SHA256: strings.Repeat("a", 64), Files: map[string]string{"browser/chrome.exe": strings.Repeat("b", 64)},
		Bin: "browser/chrome.exe", App: "browser"}
}

func TestPublishedFixtureExportsExactPin(t *testing.T) {
	var out bytes.Buffer
	release := fixture()
	if err := writeRelease(&out, release); err != nil {
		t.Fatal(err)
	}
	var got engine.Release
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Version != release.Version || got.SHA256 != release.SHA256 || got.Files[got.Bin] != release.Files[release.Bin] {
		t.Fatalf("changed pin: %+v", got)
	}
}

func TestUnpublishedOrIncompletePinNeverExports(t *testing.T) {
	cases := map[string]func(*engine.Release){
		"placeholder URL": func(r *engine.Release) { r.URL = "https://example.invalid/not-published.zip" },
		"zero archive":    func(r *engine.Release) { r.SHA256 = strings.Repeat("0", 64) },
		"zero core":       func(r *engine.Release) { r.Files[r.Bin] = strings.Repeat("0", 64) },
		"missing core":    func(r *engine.Release) { r.Files = nil },
		"invalid schema":  func(r *engine.Release) { r.URL = "http://example.com/insecure.zip" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			release := fixture()
			mutate(&release)
			var out bytes.Buffer
			if err := writeRelease(&out, release); err == nil || out.Len() != 0 {
				t.Fatalf("unsafe descriptor exported: error=%v output=%s", err, out.String())
			}
		})
	}
}
