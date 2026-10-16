// SPDX-License-Identifier: AGPL-3.0-or-later

package apostate

import (
	"github.com/D0bby999/anyprofile/internal/engine"
	"golang.org/x/sys/unix"
)

func hostMemBytes() uint64 {
	mem, _ := unix.SysctlUint64("hw.memsize")
	return mem
}

// verifyApp checks the Apple code signature and that it is Apostate's team.
func verifyApp(app string) error { return engine.VerifyCodesign(app, teamID) }
