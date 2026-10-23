// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package vm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/mobile"
)

// The runtime comes in two downloads, both through the signed engine list
// (internal/engine): "android-vm", built and signed by AnyProfile
// (scripts/build-android-vm-runtime.sh), and "android-vm-image", LineageOS
// as jqssun/android-lineage-qemu publishes it, never redistributed by
// AnyProfile: the app downloads it from there and checks the pinned hashes.
const (
	RuntimeEngine = "android-vm"
	ImageEngine   = "android-vm-image"
	// The image unpacks as a UTM bundle; the disks are in its Data folder.
	utmData = "LineageOS_on_arm64.utm/Data"
)

// ImageBuiltin is the pinned image. LineageOS 23.2 (Android 16), build
// 20261019, release v2026.10.18: run end to end on an M2 on 2026-10-23. The
// zip's hash matches GitHub's digest for the asset, and `gh attestation
// verify --repo jqssun/android-lineage-qemu` passed. "chromium" carries the
// Android version, the field every release needs. scripts/pin-android-vm-image.sh
// prints a newer entry; the steps are in docs/mobile-maintenance.md.
func ImageBuiltin() []engine.Release {
	return []engine.Release{{
		Engine: ImageEngine, Version: "23.2.20261019", Chromium: "16.0", Platform: "darwin-arm64",
		Date:   "2026-10-19",
		URL:    "https://github.com/jqssun/android-lineage-qemu/releases/download/v2026.10.18/UTM-VM-lineage-23.2-20261019-jqssun-virtio_arm64only.zip",
		SHA256: "171c0d215899d68f070fd45aea05ddb25737cf4ff16a25468e7cafbc65f4d8c9",
		Files: map[string]string{
			utmData + "/vda.qcow2": "c029184fe55eeb83556067ce64f3a6ec3efee71f4ffeff491f27944a1b22c7a6",
			utmData + "/vdb.qcow2": "de001102cbc8ff7aa8eb6c69d750cf22b94104935a0de6277ec5f24509927745",
		},
		Bin: utmData + "/vda.qcow2", Catalogue: ImageEngine,
	}}
}

// imageDisks is the folder holding an installed image's disks: the UTM
// bundle's Data folder as downloaded, or the version folder itself (the dev
// layout).
func imageDisks(dir string) string {
	if exists(filepath.Join(dir, utmData, "vda.qcow2")) {
		return filepath.Join(dir, utmData)
	}
	return dir
}

// SetCatalog gives the driver the releases it may install.
func (v *Driver) SetCatalog(c *engine.Catalog) { v.cat = c }

func (v *Driver) installedVersion(file string) string {
	b, _ := os.ReadFile(filepath.Join(v.runtime, file))
	return strings.TrimSpace(string(b))
}

// InstallStatus says what is installed and what the catalog offers.
func (v *Driver) InstallStatus() mobile.InstallStatus {
	st := mobile.InstallStatus{Runtime: mobile.Component{Engine: RuntimeEngine}, Image: mobile.Component{Engine: ImageEngine}}
	rt, err := Locate(v.runtime)
	var ae *apperr.Error
	runtimeOK := err == nil || (errors.As(err, &ae) && ae.Code == errImageMissing.Code)
	if runtimeOK {
		st.Runtime.Installed = v.installedVersion(filepath.Join("qemu", "VERSION"))
		if st.Runtime.Installed == "" {
			st.Runtime.Installed = "dev"
		}
	}
	if err == nil {
		st.Image.Installed = rt.ImageVersion
	}
	if v.cat != nil {
		if rs := v.cat.For(RuntimeEngine); len(rs) > 0 {
			st.Runtime.Latest = rs[0].Version
		}
		if rs := v.cat.For(ImageEngine); len(rs) > 0 {
			st.Image.Latest = rs[0].Version
		}
	}
	return st
}

var errNoRelease = apperr.New("android_runtime_unavailable", "the Android VM runtime is not published for this computer yet")

// Install downloads what is missing: the runtime, then the image. A runtime
// or an image already installed is kept, so an update is a new version only.
func (v *Driver) Install(ctx context.Context, progress func(engine string, done, total int64)) error {
	if v.cat == nil {
		return errNoRelease
	}
	st := v.InstallStatus()
	if st.Runtime.Installed == "" || (st.Runtime.Latest != "" && st.Runtime.Installed != "dev" && engine.NewerVersion(st.Runtime.Latest, st.Runtime.Installed)) {
		rs := v.cat.For(RuntimeEngine)
		if len(rs) == 0 {
			return errNoRelease
		}
		r := rs[0]
		dir := filepath.Join(v.runtime, "qemu")
		err := engine.InstallRelease(ctx, r, dir, func(done, total int64) { progress(RuntimeEngine, done, total) }, nil)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte(r.Version+"\n"), 0o600); err != nil {
			return err
		}
	}
	if st.Image.Installed == "" {
		rs := v.cat.For(ImageEngine)
		if len(rs) == 0 {
			return errNoRelease
		}
		r := rs[0]
		dir := filepath.Join(v.runtime, "image", r.Version)
		err := engine.InstallRelease(ctx, r, dir, func(done, total int64) { progress(ImageEngine, done, total) }, nil)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(v.runtime, "image", "CURRENT"), []byte(r.Version+"\n"), 0o600); err != nil {
			return err
		}
	}
	return nil
}
