// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package launcher

import (
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func shortTemp() string { return "" }

// listeningPorts lists the TCP ports the browser or any process it started
// listens on, from netstat's LISTENING rows (last column is the pid).
func listeningPorts(t *testing.T, pid int) string {
	out, err := exec.Command("netstat", "-ano", "-p", "TCP").Output()
	if err != nil {
		t.Fatalf("netstat: %v", err)
	}
	tree := processTree(pid)
	var rows []string
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 5 || f[3] != "LISTENING" {
			continue
		}
		if p, err := strconv.Atoi(f[4]); err == nil && slices.Contains(tree, uint32(p)) {
			rows = append(rows, strings.TrimSpace(line))
		}
	}
	return strings.Join(rows, "\n")
}

// processesUsing lists processes whose command line mentions dir.
func processesUsing(t *testing.T, dir string) string {
	script := "Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -and $_.CommandLine.Contains($env:AP_DIR) } | ForEach-Object { \"$($_.ProcessId) $($_.Name)\" }"
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = append(cmd.Environ(), "AP_DIR="+dir)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("list processes: %v", err)
	}
	return strings.TrimSpace(string(out))
}
