// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// procAttr starts the browser in its own process group, so a Ctrl+C or a
// console closing in the app's group never reaches it.
func procAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
}

// passFiles hands the child the DevTools pipes as inherited handles, named by
// --remote-debugging-io-pipes=<read>,<write> (Chromium on Windows has no fd
// 3 and 4). The app's own ends are inherited too, so the browser never sees
// EOF after the app closes them or crashes. Only those four handles are
// inherited. Data pipes (Fury's config) have no Windows path.
func passFiles(cmd *exec.Cmd, cdp, data []*os.File) error {
	if len(data) > 0 {
		return errors.New("this engine needs data pipes, which AnyProfile does not support on Windows")
	}
	if cdp == nil {
		return nil
	}
	handles := make([]syscall.Handle, len(cdp))
	for i, f := range cdp {
		h := windows.Handle(f.Fd())
		if err := windows.SetHandleInformation(h, windows.HANDLE_FLAG_INHERIT, windows.HANDLE_FLAG_INHERIT); err != nil {
			return err
		}
		handles[i] = syscall.Handle(h)
	}
	cmd.SysProcAttr.AdditionalInheritedHandles = handles
	cmd.Args = append(cmd.Args, fmt.Sprintf("--remote-debugging-io-pipes=%d,%d", handles[0], handles[1]))
	return nil
}
