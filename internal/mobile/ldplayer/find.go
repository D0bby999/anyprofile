// SPDX-License-Identifier: AGPL-3.0-or-later

package ldplayer

import "path/filepath"

// pick is the first folder holding both ldconsole.exe and adb.exe.
func pick(dirs []string, exists func(string) bool) (Install, error) {
	for _, d := range dirs {
		in := Install{Dir: d, Console: filepath.Join(d, "ldconsole.exe"), ADB: filepath.Join(d, "adb.exe")}
		if exists(in.Console) && exists(in.ADB) {
			return in, nil
		}
	}
	return Install{}, errMissing
}
