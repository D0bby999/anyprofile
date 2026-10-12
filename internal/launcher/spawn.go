// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/D0bby999/anyprofile/internal/engine"
)

// spawn execs the browser in its own process group. Each spec.Pipes[i] is
// written into a pipe the child reads as fd 3+i; the data never touches disk,
// argv or the environment. Output goes to logPath (truncated per launch), a
// file rather than a pipe so the browser survives the app crashing.
func spawn(spec engine.LaunchSpec, udd, logPath string) (int, <-chan error, error) {
	os.Remove(filepath.Join(udd, "DevToolsActivePort"))
	logf, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, nil, err
	}
	defer logf.Close()

	cmd := exec.Command(spec.Binary, spec.Args...)
	cmd.Env = append(os.Environ(), spec.Env...)
	cmd.Stdout, cmd.Stderr = logf, logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var readers, writers []*os.File
	defer func() {
		for _, f := range readers {
			f.Close()
		}
	}()
	for range spec.Pipes {
		r, w, err := os.Pipe()
		if err != nil {
			for _, f := range writers {
				f.Close()
			}
			return 0, nil, err
		}
		readers, writers = append(readers, r), append(writers, w)
	}
	cmd.ExtraFiles = readers
	if err := cmd.Start(); err != nil {
		for _, f := range writers {
			f.Close()
		}
		return 0, nil, err
	}
	for i, w := range writers {
		go func(w *os.File, data []byte) {
			w.Write(data) // the core reads to EOF; a pipe may need several reads
			w.Close()
		}(w, spec.Pipes[i])
	}
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	return cmd.Process.Pid, wait, nil
}

// hasSession reports whether Chromium has a session to restore, in which case
// the start URL is not opened again.
func hasSession(udd string) bool {
	entries, err := os.ReadDir(filepath.Join(udd, "Default", "Sessions"))
	return err == nil && len(entries) > 0
}

// readDevToolsPort waits for Chromium to write the port it bound.
func readDevToolsPort(udd string, timeout time.Duration) int {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(filepath.Join(udd, "DevToolsActivePort")); err == nil {
			if line, _, _ := strings.Cut(string(b), "\n"); line != "" {
				if port, err := strconv.Atoi(line); err == nil {
					return port
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return 0
}
