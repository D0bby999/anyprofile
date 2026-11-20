// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"os"

	"golang.org/x/sys/unix"
)

// Swap in place so an interrupted Mobile upgrade never removes the working runtime.
func publishTree(tree, dir string) error {
	if _, err := os.Lstat(dir); os.IsNotExist(err) {
		return os.Rename(tree, dir)
	} else if err != nil {
		return err
	}
	return unix.RenameatxNp(unix.AT_FDCWD, tree, unix.AT_FDCWD, dir, unix.RENAME_SWAP)
}
