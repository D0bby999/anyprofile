// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import "github.com/D0bby999/anyprofile/internal/secrets"

// furyRuns: the Fury core reads its config on extra file descriptors, which
// only the macOS launcher provides.
const furyRuns = true

// bundledConfigRel is the fury-config helper inside the app bundle, relative
// to the executable. It also serves AnyBrowser profiles (locale per country).
const bundledConfigRel = "../Resources/fury-config"

func browserCookieKey() func() ([]byte, error) {
	return func() ([]byte, error) {
		return secrets.Keychain{}.Get("Chromium Safe Storage", "Chromium")
	}
}
