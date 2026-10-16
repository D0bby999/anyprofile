// SPDX-License-Identifier: AGPL-3.0-or-later

package update

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// Apply runs the verified NSIS installer pkg silently into the folder this
// app is installed in, once pid (this app) has exited, then starts the app
// again. A hidden PowerShell does the waiting; the caller quits right after.
// Paths travel in environment variables, never through a quoted command line.
func Apply(pkg string, pid int) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	script := `Wait-Process -Id $env:AP_PID -ErrorAction SilentlyContinue
Start-Process -Wait -FilePath $env:AP_INSTALLER -ArgumentList ('/S /D=' + $env:AP_DIR)
Remove-Item -LiteralPath $env:AP_INSTALLER -ErrorAction SilentlyContinue
Start-Process -FilePath $env:AP_EXE`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	cmd.Env = append(os.Environ(), "AP_PID="+strconv.Itoa(pid), "AP_INSTALLER="+pkg,
		"AP_DIR="+filepath.Dir(exe), "AP_EXE="+exe)
	// Windowless; a Windows child outlives the parent that started it.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	return cmd.Start()
}

// CleanStale has nothing to clean on Windows: the installer replaces files
// in place and the downloaded installer deletes itself.
func CleanStale() {}
