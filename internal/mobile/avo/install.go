// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package avo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/mobile"
)

const (
	// RuntimeEngine is the Pro server runtime name for AnyMobile Metal (avo).
	RuntimeEngine = "anymobile-avo"
	// ImageEngine is the progress/status component name for the Cuttlefish AOSP image.
	ImageEngine = "anymobile-avo-image"

	// Pinned AOSP Cuttlefish arm64 Android 16 image (ANGLE 8d8796935e4f, ES 3.2 capable).
	// Downloaded directly from Google's public Android Build API v4; never redistributed
	// by AnyProfile. Verified against the pinned SHA-256 before unpacking.
	CuttlefishBuild    = "16968706"
	CuttlefishTarget   = "aosp_cf_arm64_only_phone-userdebug"
	CuttlefishArtifact = "aosp_cf_arm64_only_phone-img-16968706.zip"
	CuttlefishSHA256   = "0226a4f49ad4f080ae1f9c10d6c8e28fc3d0995fc9b2b9515958dd2ba57406ca"
	// Public AOSP build API key shipped by google/android-cuttlefish (android_build_api_key.cc).
	cuttlefishPublicAPIKey = "AIzaSyBIelMvbjtNkpa5O96eqbm_IuSUA5WsO14"
)

// Overridable in tests.
var (
	cuttlefishAPIBase = "https://androidbuild-pa.googleapis.com/v4"
	cuttlefishSHA256  = CuttlefishSHA256
)

var errNoRelease = apperr.New("anymobile_runtime_unavailable", "the Mobile Metal runtime is not published for this computer yet")

// OfferRuntime stores the signed runtime releases offered by the Pro license server.
func (d *Driver) OfferRuntime(rs []engine.Release) error {
	return d.offered.SetListed(rs)
}

// HasOffer reports whether the Pro server currently offers an anymobile-avo runtime.
func (d *Driver) HasOffer() bool {
	return len(d.offered.For(RuntimeEngine)) > 0
}

func (d *Driver) installedVersion() string {
	b, err := os.ReadFile(filepath.Join(d.avoRoot, "engine", "VERSION"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// InstallStatus reports installed and latest versions of the Metal runtime and Cuttlefish image.
func (d *Driver) InstallStatus() mobile.InstallStatus {
	st := mobile.InstallStatus{
		Runtime: mobile.Component{Engine: RuntimeEngine},
		Image:   mobile.Component{Engine: ImageEngine, Latest: CuttlefishBuild},
	}
	rt, err := Locate(d.avoRoot)
	var ae *apperr.Error
	runtimeOK := err == nil || (errors.As(err, &ae) && ae.Code == errImageMissing.Code)
	if runtimeOK {
		st.Runtime.Installed = d.installedVersion()
		if st.Runtime.Installed == "" {
			st.Runtime.Installed = "dev"
		}
	}
	if err == nil {
		st.Image.Installed = rt.ImageVersion
	}
	if rs := d.offered.For(RuntimeEngine); len(rs) > 0 {
		st.Runtime.Latest = rs[0].Version
	}
	return st
}

// Install downloads and verifies the Metal runtime (from Pro) and Cuttlefish AOSP image
// (directly from Google Android Build API v4) if missing or outdated.
func (d *Driver) Install(ctx context.Context, progress func(engine string, done, total int64)) error {
	report := func(eng string, done, total int64) {
		if progress != nil {
			progress(eng, done, total)
		}
	}
	st := d.InstallStatus()
	needsRuntime := st.Runtime.Installed == "" ||
		(st.Runtime.Latest != "" && st.Runtime.Installed != "dev" && engine.NewerVersion(st.Runtime.Latest, st.Runtime.Installed))
	if needsRuntime {
		rs := d.offered.For(RuntimeEngine)
		if len(rs) == 0 {
			return errNoRelease
		}
		r := rs[0]
		dir := filepath.Join(d.avoRoot, "engine")
		if err := engine.InstallRelease(ctx, r, dir, func(done, total int64) { report(RuntimeEngine, done, total) }, nil); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte(r.Version+"\n"), 0o600); err != nil {
			return err
		}
		st = d.InstallStatus()
	}

	if st.Image.Installed == "" {
		if err := d.installCuttlefishImage(ctx, func(done, total int64) { report(ImageEngine, done, total) }); err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) installCuttlefishImage(ctx context.Context, progress engine.Progress) error {
	signedURL, err := resolveCuttlefishURL(ctx, cuttlefishAPIBase, CuttlefishBuild)
	if err != nil {
		return fmt.Errorf("resolve Android image URL: %w", err)
	}
	imageRoot := filepath.Join(d.avoRoot, "image")
	if err := os.MkdirAll(imageRoot, 0o700); err != nil {
		return err
	}
	staging := filepath.Join(imageRoot, ".staging-"+CuttlefishBuild)
	_ = os.RemoveAll(staging)
	defer os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return err
	}

	archive := filepath.Join(staging, CuttlefishArtifact)
	if err := engine.Download(ctx, signedURL, archive, progress); err != nil {
		return fmt.Errorf("download Android image: %w", err)
	}
	if err := engine.VerifySHA256(archive, cuttlefishSHA256); err != nil {
		return fmt.Errorf("verify Android image: %w", err)
	}
	tree := filepath.Join(staging, "tree")
	if err := engine.ExtractZip(archive, tree); err != nil {
		return fmt.Errorf("extract Android image: %w", err)
	}
	_ = os.Remove(archive)

	for _, img := range []string{"boot.img", "vendor_boot.img", "super.img", "userdata.img"} {
		if !exists(filepath.Join(tree, img)) {
			return fmt.Errorf("Android image archive missing %s", img)
		}
	}
	dst := filepath.Join(imageRoot, CuttlefishBuild)
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := os.Rename(tree, dst); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(imageRoot, "current"), []byte(CuttlefishBuild+"\n"), 0o600)
}

func resolveCuttlefishURL(ctx context.Context, apiBase, buildID string) (string, error) {
	u := fmt.Sprintf("%s/builds/%s/%s/attempts/latest/artifacts/%s/url",
		strings.TrimRight(apiBase, "/"), buildID, CuttlefishTarget, CuttlefishArtifact)
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-goog-api-key", cuttlefishPublicAPIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("android build API returned %d", resp.StatusCode)
	}
	var payload struct {
		SignedURL string `json:"signedUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.SignedURL == "" {
		return "", errors.New("android build API returned empty signedUrl")
	}
	return payload.SignedURL, nil
}
