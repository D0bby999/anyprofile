// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !dev

package main

// Release builds only trust the copy inside their own bundle.
func devConfigCandidates(string) []string { return nil }
