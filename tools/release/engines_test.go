// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"crypto/ed25519"
	"testing"

	"github.com/D0bby999/anyprofile/internal/update"
)

// engines.json is signed into every latest.json: a release the app would
// refuse must fail here, not in the release workflow.
func TestEnginesJSONSigns(t *testing.T) {
	_, key, _ := ed25519.GenerateKey(nil)
	raw, sig, err := signEngines(key, "../../engines.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := update.EnginesMessage(raw); err != nil || sig == "" {
		t.Fatalf("message: %v", err)
	}
}
