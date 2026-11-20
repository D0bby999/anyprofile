// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

// Package avo runs AnyMobile devices on macOS via AnyProfile's Metal-accelerated
// Rust Hypervisor.framework VMM (anyprofile-vmm), gfxstream + MoltenVK, AOSP
// Cuttlefish arm64 images, and native Cocoa shared-memory viewports (anyprofile-view).
package avo

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

// Runtime describes an installed Avo engine and Cuttlefish AOSP image set.
type Runtime struct {
	Root         string
	VMM          string
	View         string
	ADB          string
	GfxstreamDir string
	ProfilesDir  string
	LayersDir    string
	ImageVersion string
	ImageDir     string
}

var (
	errRuntimeMissing = apperr.New("anymobile_runtime_missing", "the Metal Mobile runtime (avo) is not staged")
	errImageMissing   = apperr.New("anymobile_image_missing", "the Cuttlefish Android image is missing")
)

// RootFor returns the Avo runtime root next to AnyProfile's qemu runtime
// directory (~/Library/Application Support/AnyProfile/mobile/avo), or the
// ANYPROFILE_AVO_DIR override when set.
func RootFor(qemuRuntimeDir string) string {
	if env := strings.TrimSpace(os.Getenv("ANYPROFILE_AVO_DIR")); env != "" {
		return env
	}
	return filepath.Join(filepath.Dir(qemuRuntimeDir), "avo")
}

// Staged reports whether the Metal Avo engine and Cuttlefish image are ready.
func Staged(avoRoot string) bool {
	_, err := Locate(avoRoot)
	return err == nil
}

// Locate checks that every required binary, gfxstream library, and Cuttlefish
// image file exists under avoRoot. Binaries live under avoRoot/engine when
// installed by AnyProfile's updater, or directly under avoRoot in dev staging.
func Locate(avoRoot string) (Runtime, error) {
	engRoot := avoRoot
	if exists(filepath.Join(avoRoot, "engine", "bin", "anyprofile-vmm")) {
		engRoot = filepath.Join(avoRoot, "engine")
	}
	r := Runtime{
		Root:         avoRoot,
		VMM:          filepath.Join(engRoot, "bin", "anyprofile-vmm"),
		View:         filepath.Join(engRoot, "bin", "anyprofile-view"),
		ADB:          filepath.Join(engRoot, "bin", "adb"),
		GfxstreamDir: filepath.Join(engRoot, "gfxstream"),
		ProfilesDir:  filepath.Join(engRoot, "profiles"),
		LayersDir:    filepath.Join(engRoot, "layers"),
	}
	for _, p := range []string{
		r.VMM,
		r.View,
		r.ADB,
		filepath.Join(r.GfxstreamDir, "lib64", "libgfxstream_backend.dylib"),
	} {
		if !exists(p) {
			return r, errRuntimeMissing
		}
	}
	b, err := os.ReadFile(filepath.Join(avoRoot, "image", "current"))
	ver := strings.TrimSpace(string(b))
	if err != nil || ver == "" || strings.ContainsAny(ver, `/\`) || ver == "." || ver == ".." {
		return r, errImageMissing
	}
	r.ImageVersion = ver
	r.ImageDir = filepath.Join(avoRoot, "image", ver)
	for _, img := range []string{"boot.img", "vendor_boot.img", "super.img", "userdata.img"} {
		if !exists(filepath.Join(r.ImageDir, img)) {
			return r, errImageMissing
		}
	}
	return r, nil
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
