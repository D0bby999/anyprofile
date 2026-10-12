// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import "os/exec"

func execOutput(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return string(out), err
}
