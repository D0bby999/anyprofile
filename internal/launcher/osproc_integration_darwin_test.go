// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package launcher

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// shortTemp keeps test data under /tmp: Chromium's SingletonSocket path must
// stay under the 104-byte limit of a Unix socket name.
func shortTemp() string { return "/tmp" }

// listeningPorts lists the TCP ports the browser's process group listens on.
func listeningPorts(t *testing.T, pid int) string {
	out, _ := exec.Command("lsof", "-nP", "-a", "-iTCP", "-sTCP:LISTEN", "-g", strconv.Itoa(pid)).Output()
	return strings.TrimSpace(string(out))
}

// processesUsing lists processes whose command line mentions dir.
func processesUsing(t *testing.T, dir string) string {
	out, _ := exec.Command("pgrep", "-f", dir).Output()
	return strings.TrimSpace(string(out))
}
