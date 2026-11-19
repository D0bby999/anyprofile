// SPDX-License-Identifier: AGPL-3.0-or-later

// Package mumu12 drives MuMuPlayer 12 (Android 12, DirectX/Vulkan GPU) on
// Windows through MuMuManager.exe, giving Windows users a modern alternative
// to LDPlayer 9 (Android 9) while sharing AnyProfile's ADB control, sync,
// macro and proxy architecture.
package mumu12

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

var errMissing = apperr.New("mumu12_missing", "MuMuPlayer 12 is not installed on this computer")

// Info is the subset of `MuMuManager.exe info -v <index>` JSON we consume.
type Info struct {
	Index            int    `json:"index"`
	Name             string `json:"name"`
	ADBPort          int    `json:"adb_port"`
	IsProcessStarted bool   `json:"is_process_started"`
	IsAndroidStarted bool   `json:"is_android_started"`
	PID              int    `json:"pid"`
}

type manager struct {
	dir string // folder holding MuMuManager.exe and adb.exe
}

func (m manager) bin() string { return filepath.Join(m.dir, "MuMuManager.exe") }
func (m manager) adb() string { return filepath.Join(m.dir, "adb.exe") }

func (m manager) run(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, m.bin(), args...).CombinedOutput()
	s := strings.TrimSpace(string(bytes.ReplaceAll(out, []byte("\r"), nil)))
	if err != nil {
		return s, fmt.Errorf("MuMuManager %s: %v: %s", strings.Join(args, " "), err, s)
	}
	return s, nil
}

// parseInfo parses either a single object or a map of index -> Info returned by
// `MuMuManager.exe info -v all` or `-v <idx>`.
func parseInfo(raw string) ([]Info, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var single Info
	if json.Unmarshal([]byte(raw), &single) == nil && single.Name != "" {
		return []Info{single}, nil
	}
	var byIdx map[string]Info
	if err := json.Unmarshal([]byte(raw), &byIdx); err != nil {
		return nil, err
	}
	out := make([]Info, 0, len(byIdx))
	for k, v := range byIdx {
		if v.Index == 0 {
			if n, err := strconv.Atoi(k); err == nil {
				v.Index = n
			}
		}
		out = append(out, v)
	}
	return out, nil
}

// settingArgs builds `MuMuManager.exe setting -v <idx> ...` flags for CPU, RAM
// and iPhone-proportion screen dimensions.
func settingArgs(idx, cpus, ramMB, w, h, dpi int) []string {
	args := []string{"setting", "-v", strconv.Itoa(idx)}
	if cpus > 0 {
		args = append(args, "-k", "performance_cpu.custom", "-val", strconv.Itoa(cpus))
	}
	if ramMB > 0 {
		args = append(args, "-k", "performance_mem.custom", "-val", strconv.Itoa(max(ramMB/1024, 2)))
	}
	if w > 0 && h > 0 {
		args = append(args, "-k", "resolution_mode", "-val", "custom",
			"-k", "resolution_width.custom", "-val", strconv.Itoa(w),
			"-k", "resolution_height.custom", "-val", strconv.Itoa(h))
	}
	if dpi > 0 {
		args = append(args, "-k", "resolution_dpi.custom", "-val", strconv.Itoa(dpi))
	}
	return args
}

func validateIndex(ref string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(ref))
	if err != nil || n < 0 {
		return 0, errors.New("invalid MuMuPlayer 12 instance index")
	}
	return n, nil
}
