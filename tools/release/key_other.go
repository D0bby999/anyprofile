// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !darwin

package main

import "errors"

func loadSeed() ([]byte, error) {
	return nil, errors.New("keep the release key on a Mac, or set UPDATE_SIGNING_KEY")
}
func saveSeed([]byte) error { return errors.New("make the release key on a Mac") }
