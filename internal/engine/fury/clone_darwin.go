// SPDX-License-Identifier: AGPL-3.0-or-later

package fury

import "golang.org/x/sys/unix"

func clone(src, dst string) error { return unix.Clonefile(src, dst, 0) }
