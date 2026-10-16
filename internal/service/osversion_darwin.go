// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import "golang.org/x/sys/unix"

func osVersion() string {
	v, _ := unix.Sysctl("kern.osproductversion")
	return v
}
