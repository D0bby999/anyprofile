// SPDX-License-Identifier: AGPL-3.0-or-later

package ldplayer

import (
	"os"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// registryKeys are where LDPlayer 9's installer records its folder (to be
// confirmed on a real install, phase 1 of the mobile plan).
var registryKeys = []string{`Software\leidian\ldplayer9`, `Software\XuanZhi\LDPlayer9`}

// Find locates LDPlayer 9: the folder its installer recorded, else the
// default install folder.
func Find() (Install, error) {
	var dirs []string
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		for _, path := range registryKeys {
			k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			if v, _, err := k.GetStringValue("InstallDir"); err == nil && v != "" {
				dirs = append(dirs, v)
			}
			k.Close()
		}
	}
	dirs = append(dirs, `C:\LDPlayer\LDPlayer9`)
	return pick(dirs, func(p string) bool { _, err := os.Stat(p); return err == nil })
}

func hidden() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}
