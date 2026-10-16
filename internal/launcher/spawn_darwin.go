// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"os"
	"os/exec"
	"syscall"
)

func procAttr() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setpgid: true} }

// passFiles hands the child its pipes as file descriptors: the DevTools
// command/reply pipes on fds 3 and 4 (what --remote-debugging-pipe reads),
// then the data pipes from fd 5 (3 without DevTools), then the app's own ends
// of the DevTools pipes, which keep the browser from ever seeing EOF after
// the app closes them or crashes. cdp is {cmdR, respW, cmdW, respR} or nil.
func passFiles(cmd *exec.Cmd, cdp, data []*os.File) error {
	if cdp != nil {
		cmd.ExtraFiles = append(append([]*os.File{cdp[0], cdp[1]}, data...), cdp[2], cdp[3])
		return nil
	}
	cmd.ExtraFiles = data
	return nil
}
