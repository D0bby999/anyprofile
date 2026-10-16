// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import "github.com/D0bby999/anyprofile/internal/secrets"

// The release key lives in the login Keychain of the release Mac.
const keyService, keyAccount = "dev.anyprofile.release-key", "release"

func loadSeed() ([]byte, error)  { return secrets.Keychain{}.Get(keyService, keyAccount) }
func saveSeed(seed []byte) error { return secrets.Keychain{}.Set(keyService, keyAccount, seed) }
