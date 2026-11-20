// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !darwin

package engine

import "os"

func publishTree(tree, dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return os.Rename(tree, dir)
}
