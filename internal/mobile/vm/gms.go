// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package vm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/adb"
)

// Pinned upstream packages for the GMS + Magisk base layer (Android 16 /
// LineageOS 23.2, matching ImageBuiltin). AnyProfile never redistributes
// proprietary Google binaries; they are downloaded directly from upstream,
// verified by SHA-256, and sideloaded locally once into vda-gms.qcow2.
const (
	mindTheGappsName = "MindTheGapps-16.0.0-arm64-20261023_034858.zip"
	mindTheGappsURL  = "https://github.com/MindTheGapps/16.0.0-arm64/releases/download/MindTheGapps-16.0.0-arm64-20261023_034858/" + mindTheGappsName
	mindTheGappsSHA  = "6929593937cd85541228e4e5db9cbb5455338bf33c75eabf610de632ac155810"

	recoveryDebugName = "recovery_arm64only-userdebug-v2026.10.18.img"
	recoveryDebugURL  = "https://github.com/jqssun/android-lineage-qemu/releases/download/v2026.10.18/recovery_arm64only-userdebug.img"
	recoveryDebugSHA  = "18e84a5a577613c8dce07bd078d243de3b120b21a441aa0863a3144a787856df"

	magiskAPKName = "Magisk-v30.7.apk"
	magiskAPKURL  = "https://github.com/topjohnwu/Magisk/releases/download/v30.7/" + magiskAPKName
	magiskAPKSHA  = "e0d32d2123532860f97123d927b1bb86c4e08e6fd8a48bfc6b5bee0afae9ebd5"

	gmsReadyStamp = mindTheGappsSHA + "\n" + magiskAPKSHA + "\n"
)

// gmsBaseDir holds the prepared GMS + Magisk base overlay next to the stock image.
func gmsBaseDir(rt Runtime) string { return filepath.Join(rt.ImageDir, "gms") }

// gmsBaseDisk returns the prepared vda-gms.qcow2 path when READY matches the
// current pinned packages.
func gmsBaseDisk(rt Runtime) (string, bool) {
	dir := gmsBaseDir(rt)
	b, err := os.ReadFile(filepath.Join(dir, "READY"))
	if err != nil || string(b) != gmsReadyStamp {
		return "", false
	}
	p := filepath.Join(dir, "vda-gms.qcow2")
	if !exists(p) {
		return "", false
	}
	return p, true
}

// writePartition writes a raw partition image (such as recovery-userdebug.img)
// directly into the named GPT partition of a stopped qcow2 disk.
func writePartition(ctx context.Context, d disk, partName, imgPath string) error {
	gpt, err := d.readRaw(ctx, 0, gptHeader)
	if err != nil {
		return err
	}
	first, sectors, err := partition(gpt, partName)
	if err != nil {
		return err
	}
	info, err := os.Stat(imgPath)
	if err != nil {
		return err
	}
	partBytes := sectors * sector
	if info.Size() > partBytes {
		return fmt.Errorf("%s (%d bytes) exceeds partition %s (%d bytes)", filepath.Base(imgPath), info.Size(), partName, partBytes)
	}
	opts := fmt.Sprintf("driver=raw,offset=%d,size=%d,file.driver=qcow2,file.file.filename=%s",
		first*sector, info.Size(), optEscape(d.path))
	if out, err := exec.CommandContext(ctx, d.qemuImg, "convert", "-n", "-f", "raw", imgPath, "--target-image-opts", opts).CombinedOutput(); err != nil {
		return fmt.Errorf("write partition %s: %v: %s", partName, err, bytes.TrimSpace(out))
	}
	return nil
}

// writeMiscBCB writes an Android Bootloader Control Block (BCB) into the misc
// partition. With command="boot-recovery" and recoveryArgs="recovery\n--sideload_auto_reboot\n",
// GRUB boots LineageOS Recovery directly into ADB sideload mode with no UI clicks.
// Passing empty command clears the BCB for normal Android boot.
func writeMiscBCB(ctx context.Context, d disk, command, recoveryArgs string) error {
	gpt, err := d.readRaw(ctx, 0, gptHeader)
	if err != nil {
		return err
	}
	first, sectors, err := partition(gpt, "misc")
	if err != nil {
		return err
	}
	if sectors*sector < 2048 {
		return errors.New("misc partition too small for BCB")
	}
	bcb := make([]byte, 2048)
	if len(command) >= 32 || len(recoveryArgs) >= 768 {
		return errors.New("BCB payload too large")
	}
	copy(bcb[0:32], command)
	copy(bcb[64:832], recoveryArgs)
	return d.writeRaw(ctx, first*sector, bcb)
}

// PrepareGMSBase builds (once per image version) a qcow2 base overlay with
// MindTheGapps (Google Play Services + Play Store for YouTube) and Magisk
// (for per-device identity spoofing) sideloaded via LineageOS Recovery.
// Subsequent devices clone this base in <1s without repeating sideload.
func (v *Driver) PrepareGMSBase(ctx context.Context, rt Runtime) (string, error) {
	if p, ok := gmsBaseDisk(rt); ok {
		return p, nil
	}
	v.gmsMu.Lock()
	defer v.gmsMu.Unlock()
	if p, ok := gmsBaseDisk(rt); ok {
		return p, nil
	}

	cacheDir := filepath.Join(v.runtime, "cache")
	v.log.Info("preparing AnyMobile GMS + Magisk base image (one-time setup)")
	recPath, err := adb.FetchAPK(ctx, cacheDir, recoveryDebugName, recoveryDebugURL, recoveryDebugSHA)
	if err != nil {
		return "", fmt.Errorf("download recovery image: %w", err)
	}
	gappsPath, err := adb.FetchAPK(ctx, cacheDir, mindTheGappsName, mindTheGappsURL, mindTheGappsSHA)
	if err != nil {
		return "", fmt.Errorf("download MindTheGapps: %w", err)
	}
	magiskPath, err := adb.FetchAPK(ctx, cacheDir, magiskAPKName, magiskAPKURL, magiskAPKSHA)
	if err != nil {
		return "", fmt.Errorf("download Magisk: %w", err)
	}

	outDir := gmsBaseDir(rt)
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}
	workDir, err := os.MkdirTemp(outDir, "build-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(workDir)

	vda := filepath.Join(workDir, "vda.qcow2")
	vdb := filepath.Join(workDir, "vdb.qcow2")
	efi := filepath.Join(workDir, "efi_vars.fd")
	if out, err := exec.CommandContext(ctx, rt.QEMUImg, "create", "-q", "-f", "qcow2", "-F", "qcow2",
		"-b", filepath.Join(rt.ImageDir, "vda.qcow2"), vda).CombinedOutput(); err != nil {
		return "", fmt.Errorf("create gms overlay: %v: %s", err, bytes.TrimSpace(out))
	}
	if err := copyFile(filepath.Join(rt.ImageDir, "vdb.qcow2"), vdb); err != nil {
		return "", err
	}
	if err := cloneOrCopy(rt.FirmwareVars, efi); err != nil {
		return "", err
	}
	dsk := disk{qemuImg: rt.QEMUImg, path: vda}
	if err := setGrubenv(ctx, dsk, grubDefaults); err != nil {
		return "", fmt.Errorf("gms grubenv: %w", err)
	}
	// Flash userdebug recovery so LineageOS Recovery accepts unsigned/third-party zips.
	if err := writePartition(ctx, dsk, "recovery", recPath); err != nil {
		return "", fmt.Errorf("flash userdebug recovery: %w", err)
	}

	// Sideload MindTheGapps, then Magisk, each in a clean offline recovery boot.
	for _, step := range []struct {
		name string
		zip  string
	}{
		{"MindTheGapps", gappsPath},
		{"Magisk", magiskPath},
	} {
		if err := v.sideloadInRecovery(ctx, rt, workDir, dsk, step.name, step.zip); err != nil {
			return "", fmt.Errorf("sideload %s: %w", step.name, err)
		}
	}

	// Clear BCB so devices cloned from vda-gms.qcow2 boot normally into Android.
	if err := writeMiscBCB(ctx, dsk, "", ""); err != nil {
		return "", fmt.Errorf("clear misc BCB: %w", err)
	}

	finalDisk := filepath.Join(outDir, "vda-gms.qcow2")
	tmpFinal := finalDisk + ".part"
	os.Remove(tmpFinal)
	if err := os.Rename(vda, tmpFinal); err != nil {
		return "", err
	}
	if err := os.Rename(tmpFinal, finalDisk); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(outDir, "READY"), []byte(gmsReadyStamp), 0o600); err != nil {
		return "", err
	}
	v.log.Info("AnyMobile GMS + Magisk base image ready", "path", finalDisk)
	return finalDisk, nil
}

// sideloadInRecovery boots workDir into LineageOS Recovery sideload mode,
// pushes zipPath with `adb sideload`, and stops QEMU cleanly.
func (v *Driver) sideloadInRecovery(ctx context.Context, rt Runtime, workDir string, dsk disk, label, zipPath string) error {
	if err := writeMiscBCB(ctx, dsk, "boot-recovery", "recovery\n--sideload\n"); err != nil {
		return err
	}
	dev := mobile.Device{ID: "gms-prep", Name: "gms-prep", RAMMB: 3072, CPUs: 4}
	netSock, qmpSock, err := sockets("gms-prep-" + strings.ToLower(label))
	if err != nil {
		return err
	}
	defer os.Remove(netSock)
	defer os.Remove(qmpSock)

	p, err := v.startRawQEMU(ctx, rt, dev, workDir, offline, netSock, qmpSock)
	if err != nil {
		return err
	}
	defer v.kill(p)

	wctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	err = p.adb.WaitSideload(wctx, p.exited)
	cancel()
	if err != nil {
		return err
	}

	sctx, scancel := context.WithTimeout(ctx, 8*time.Minute)
	out, err := p.adb.Run(sctx, "sideload", zipPath)
	scancel()
	// adb sideload returns exit 0 (or "Total xfer: 1.00x" / "Total xfer: 2.00x") on success.
	if err != nil && !strings.Contains(out, "Total xfer:") {
		return fmt.Errorf("%w (%s)", err, out)
	}
	// Give recovery a brief moment to flush ext4/super writes before shutting down.
	time.Sleep(3 * time.Second)
	return nil
}
