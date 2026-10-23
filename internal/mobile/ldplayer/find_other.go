// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package ldplayer

import "syscall"

// Find: LDPlayer runs on Windows only.
func Find() (Install, error) { return Install{}, errMissing }

func hidden() *syscall.SysProcAttr { return nil }
