// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package adb

import "syscall"

func hidden() *syscall.SysProcAttr { return nil }
