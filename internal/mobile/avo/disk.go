// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package avo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/adb"
	"github.com/D0bby999/anyprofile/internal/mobile/vm"
)

const cloneReinitMarker = ".clone_reinit"

// CloneDevice creates a near-instant APFS copy-on-write clone of a stopped
// Golden Master device's disks and marks the clone so its first boot generates
// a fresh Android ID / SSAID (and optionally clears Google Services IDs) while
// preserving all installed apps.
func (d *Driver) CloneDevice(ctx context.Context, src, dst mobile.Device, resetGoogleIDs bool) (string, string, error) {
	if d.proc(src.ID) != nil {
		return "", "", errors.New("stop the source device before cloning")
	}
	rt, err := Locate(d.avoRoot)
	if err != nil {
		return "", "", err
	}
	srcInst := d.instDir(src)
	if !exists(filepath.Join(srcInst, "super.img")) {
		return "", "", errors.New("source device disks are not initialized yet")
	}
	dstInst := d.instDir(dst)
	if err := os.MkdirAll(dstInst, 0o700); err != nil {
		return "", "", err
	}
	for _, f := range instanceDisks {
		sPath := filepath.Join(srcInst, f)
		if !exists(sPath) {
			_ = os.RemoveAll(d.dir(dst))
			return "", "", fmt.Errorf("source device is missing disk %s", f)
		}
		if err := vm.CloneOrCopy(sPath, filepath.Join(dstInst, f)); err != nil {
			_ = os.RemoveAll(d.dir(dst))
			return "", "", err
		}
	}
	for _, marker := range []string{".gpu_layers_ready", ".home_layout_ready"} {
		if b, err := os.ReadFile(filepath.Join(srcInst, marker)); err == nil {
			_ = os.WriteFile(filepath.Join(dstInst, marker), b, 0o600)
		}
	}
	mode := "ids\n"
	if resetGoogleIDs {
		mode = "full\n"
	}
	if err := os.WriteFile(filepath.Join(dstInst, cloneReinitMarker), []byte(mode), 0o600); err != nil {
		_ = os.RemoveAll(d.dir(dst))
		return "", "", err
	}
	_ = ctx
	return dst.ID, "cuttlefish-" + rt.ImageVersion, nil
}

// applyCloneReinit runs once on a cloned device's first boot so it never shares
// the Golden Master's Android ID, app-scoped SSAIDs, or Google Play IDs.
func applyCloneReinit(ctx context.Context, a adb.Client, instDir string, log *slog.Logger) {
	if instDir == "" {
		return
	}
	marker := filepath.Join(instDir, cloneReinitMarker)
	b, err := os.ReadFile(marker)
	if err != nil {
		return
	}
	mode := strings.TrimSpace(string(b))
	var rnd [8]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return
	}
	newAndroidID := hex.EncodeToString(rnd[:])
	script := fmt.Sprintf(
		"settings put secure android_id %s; rm -f /data/system/users/0/settings_ssaid.xml",
		newAndroidID,
	)
	if mode == "full" {
		script += "; pm clear com.google.android.gsf >/dev/null 2>&1 || true; pm clear com.google.android.gms >/dev/null 2>&1 || true; pm clear com.android.vending >/dev/null 2>&1 || true"
	}
	c, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	if _, err := a.Shell(c, "su 0 sh -c '"+script+"'"); err != nil {
		if _, err2 := a.Shell(c, script); err2 != nil {
			log.Warn("avo clone reinit", "err", err)
			return
		}
	}
	_ = os.Remove(marker)
}

// PhysicalDiskBytes returns the actual APFS allocated bytes (st_blocks * 512)
// used by the device's instance disks, not their logical sparse size.
func (d *Driver) PhysicalDiskBytes(dev mobile.Device) int64 {
	entries, err := os.ReadDir(d.instDir(dev))
	if err != nil {
		return 0
	}
	var total int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		var st syscall.Stat_t
		if err := syscall.Stat(filepath.Join(d.instDir(dev), e.Name()), &st); err == nil {
			total += st.Blocks * 512
		}
	}
	return total
}

// CompactDisk reclaims wasted cache and sparse disk blocks without losing apps
// or logins. On a running device it trims app caches and invokes Android fstrim;
// on a stopped device it punches APFS sparse holes over zeroed 1 MiB extents of
// userdata.img.
func (d *Driver) CompactDisk(ctx context.Context, dev mobile.Device) (beforeBytes, afterBytes int64, err error) {
	beforeBytes = d.PhysicalDiskBytes(dev)
	if p := d.proc(dev.ID); p != nil {
		c, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		const cmd = "pm trim-caches 999G >/dev/null 2>&1 || true; " +
			"rm -rf /data/local/tmp/*.apk /data/tombstones/* /data/anr/* /sdcard/Android/data/*/cache/* 2>/dev/null || true; " +
			"sm fstrim >/dev/null 2>&1 || true; sync"
		if _, err := p.adb.Shell(c, "su 0 sh -c '"+cmd+"'"); err != nil {
			_, _ = p.adb.Shell(c, cmd)
		}
	} else {
		ud := filepath.Join(d.instDir(dev), "userdata.img")
		if exists(ud) {
			if perr := punchZeroHoles(ud); perr != nil {
				return beforeBytes, beforeBytes, perr
			}
		}
	}
	afterBytes = d.PhysicalDiskBytes(dev)
	return beforeBytes, afterBytes, nil
}

// punchZeroHoles scans path in 1 MiB chunks and asks APFS (F_PUNCHHOLE) to
// deallocate runs of pure zero bytes without changing logical file contents.
func punchZeroHoles(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return err
	}
	size := st.Size()
	if size == 0 {
		return nil
	}
	const chunkSize int64 = 1 << 20 // 1 MiB alignment matches APFS extents well
	buf := make([]byte, chunkSize)
	var holeStart int64 = -1

	flushHole := func(end int64) {
		if holeStart < 0 || end <= holeStart {
			return
		}
		// macOS <sys/fcntl.h> struct fpunchhole for F_PUNCHHOLE.
		var ph struct {
			Flags    uint32
			Reserved uint32
			Offset   int64
			Length   int64
		}
		ph.Offset = holeStart
		ph.Length = end - holeStart
		_, _, _ = unix.Syscall(unix.SYS_FCNTL, f.Fd(), uintptr(unix.F_PUNCHHOLE), uintptr(unsafe.Pointer(&ph)))
		holeStart = -1
	}

	var off int64
	for off < size {
		n, rerr := f.ReadAt(buf, off)
		if n > 0 && int64(n) == chunkSize && isAllZero(buf[:n]) {
			if holeStart < 0 {
				holeStart = off
			}
		} else {
			flushHole(off)
		}
		off += int64(n)
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	flushHole(off)
	return nil
}

func isAllZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}
