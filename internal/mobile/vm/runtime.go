// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package vm

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

// Runtime is what a VM needs on disk. Under the mobile runtime folder:
//
//	qemu/bin/qemu-system-aarch64, qemu/bin/qemu-img      the "android-vm" package
//	qemu/share/edk2-aarch64-code.fd, qemu/share/edk2-arm-vars.fd (a device's EFI variables start as this)
//	qemu/bin/adb, qemu/bin/scrcpy, qemu/share/scrcpy-server
//	image/CURRENT                                         the image version in use
//	image/<version>/vda.qcow2, vdb.qcow2                  the "android-vm-image" package
//
// scripts/dev-android-vm-runtime.sh lays the same tree out from Homebrew and
// a downloaded image for development.
type Runtime struct {
	QEMU, QEMUImg, Firmware   string
	FirmwareVars              string
	ADB, Scrcpy, ScrcpyServer string
	ImageVersion, ImageDir    string
}

var (
	errRuntimeMissing = apperr.New("android_runtime_missing", "the Android VM runtime is not installed; install it in Settings")
	errImageMissing   = apperr.New("android_image_missing", "the Android image is not installed; install it in Settings")
)

// Locate finds the runtime under root, or says what is missing.
func Locate(root string) (Runtime, error) {
	q := filepath.Join(root, "qemu")
	r := Runtime{
		QEMU:         filepath.Join(q, "bin", "qemu-system-aarch64"),
		QEMUImg:      filepath.Join(q, "bin", "qemu-img"),
		Firmware:     filepath.Join(q, "share", "edk2-aarch64-code.fd"),
		FirmwareVars: filepath.Join(q, "share", "edk2-arm-vars.fd"),
		ADB:          filepath.Join(q, "bin", "adb"),
		Scrcpy:       filepath.Join(q, "bin", "scrcpy"),
		ScrcpyServer: filepath.Join(q, "share", "scrcpy-server"),
	}
	for _, p := range []string{r.QEMU, r.QEMUImg, r.Firmware, r.FirmwareVars, r.ADB, r.Scrcpy, r.ScrcpyServer} {
		if !exists(p) {
			return r, errRuntimeMissing
		}
	}
	b, err := os.ReadFile(filepath.Join(root, "image", "CURRENT"))
	ver := strings.TrimSpace(string(b))
	if err != nil || ver == "" || strings.ContainsAny(ver, `/\`) || ver == "." || ver == ".." {
		return r, errImageMissing
	}
	r.ImageVersion, r.ImageDir = ver, imageDisks(filepath.Join(root, "image", ver))
	for _, f := range []string{"vda.qcow2", "vdb.qcow2"} {
		if !exists(filepath.Join(r.ImageDir, f)) {
			return r, errImageMissing
		}
	}
	return r, nil
}

// imageFiles are a device's disks and EFI variables, what a backup holds.
var imageFiles = []string{"vda.qcow2", "vdb.qcow2", "efi_vars.fd"}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
