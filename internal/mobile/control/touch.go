// SPDX-License-Identifier: AGPL-3.0-or-later

package control

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

// LongPress holds a finger on a ref or a point long enough for Android to
// open a context menu. 700-900 ms is the reliable range; 3000 ms is the cap.
func (d *Device) LongPress(ctx context.Context, ref string, x, y, ms int) error {
	d.wake(ctx)
	if ref != "" {
		r, err := d.rect(ref)
		if err != nil {
			return err
		}
		x, y = r.Center()
	}
	if x <= 0 || y <= 0 {
		return apperr.New("mobile_point", "give a ref, or x and y")
	}
	if ms <= 0 || ms > 3000 {
		ms = 800
	}
	_, err := d.shell(ctx, fmt.Sprintf("input swipe %d %d %d %d %d", x, y, x, y, ms))
	return err
}

// Drag moves a finger from one point to another. Lists and launchers see it as
// a drag, not a scroll, because the move takes 400-900 ms.
func (d *Device) Drag(ctx context.Context, x1, y1, x2, y2, ms int) error {
	d.wake(ctx)
	if x1 <= 0 || y1 <= 0 || x2 <= 0 || y2 <= 0 {
		return apperr.New("mobile_point", "give both the start and the end point")
	}
	if ms <= 0 || ms > 3000 {
		ms = 600
	}
	_, err := d.shell(ctx, fmt.Sprintf("input swipe %d %d %d %d %d", x1, y1, x2, y2, ms))
	return err
}

// pullDirs are the only device folders a file may be pulled from. A path
// outside them is rejected: the agent must not read arbitrary device storage.
var pullDirs = []string{"/sdcard/DCIM/AnyProfile/", "/sdcard/Download/AnyProfile/", "/sdcard/Pictures/AnyProfile/"}

// Pull copies a file from an allowlisted device folder to
// ~/Downloads/AnyProfile and returns the local path.
func (d *Device) Pull(ctx context.Context, remote string) (string, error) {
	if strings.Contains(remote, "..") {
		return "", apperr.New("mobile_pull_path", "the path must not contain ..")
	}
	clean := remote
	if !strings.HasPrefix(clean, "/") {
		clean = "/" + clean
	}
	allowed := false
	for _, dir := range pullDirs {
		if strings.HasPrefix(clean, dir) && len(clean) > len(dir) {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", apperr.New("mobile_pull_path", "only files under /sdcard/DCIM/AnyProfile, /sdcard/Download/AnyProfile or /sdcard/Pictures/AnyProfile can be pulled")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Downloads", "AnyProfile")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	local := filepath.Join(dir, filepath.Base(clean))
	if _, err := d.ADB.Run(ctx, "pull", clean, local); err != nil {
		return "", err
	}
	return local, nil
}
