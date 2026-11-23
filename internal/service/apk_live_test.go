// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin && avointegration

// Run real end-to-end APK URL download, 32 GB APK Runner install, uncataloged
// app detection, ImportInstalledAPK pull, and Aurora Store deep-link:
//
//	go test -tags avointegration -run TestAPKStoreLiveDownloadAndImport -v -timeout 10m ./internal/service/
package service

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/avo"
)

func TestAPKStoreLiveDownloadAndImport(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	qemuRuntime := filepath.Join(home, "Library", "Application Support", "AnyProfile", "mobile", "runtime")
	if !avo.Staged(avo.RootFor(qemuRuntime)) {
		t.Skip("avo runtime not staged")
	}

	s, _ := newMobileService(t, "active")
	devicesDir := t.TempDir()
	drv := avo.New(qemuRuntime, devicesDir, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	s.Mobile = mobile.NewManager(drv, s.Store, s.Log, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	t.Cleanup(func() {
		if id, ok := s.apkHostID(""); ok && s.mobileRunning(id) {
			sctx, scancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer scancel()
			_ = s.StopMobileDevice(sctx, id)
		}
	})

	// 1. Real HTTPS download of official Telegram APK + offline InspectAPK.
	t.Log("step 1: downloading real Telegram APK from https://telegram.org/dl/android/apk")
	dlInfo, err := s.DownloadAPKFromURL(ctx, "https://telegram.org/dl/android/apk", "")
	if err != nil {
		t.Fatalf("DownloadAPKFromURL: %v", err)
	}
	t.Logf("downloaded: pkg=%s label=%q ver=%s size=%d abi=%v compat=%v icon=%dB",
		dlInfo.Package, dlInfo.Label, dlInfo.VersionName, dlInfo.SizeBytes, dlInfo.ABIs, dlInfo.ABICompatible, len(dlInfo.IconData))
	if !strings.Contains(dlInfo.Package, "telegram") || !dlInfo.ABICompatible {
		t.Fatalf("unexpected Telegram inspect: %+v", dlInfo)
	}

	// 2. Install onto real 32 GB APK Runner via AddAPK.
	t.Log("step 2: installing downloaded APK onto 32 GB APK Runner")
	rec, err := s.AddAPK(ctx, APKInput{
		FilePath:       dlInfo.FilePath,
		Orientation:    "portrait",
		CreateShortcut: false,
	})
	if err != nil {
		t.Fatalf("AddAPK: %v", err)
	}
	t.Logf("installed: pkg=%s label=%q cache=%s device=%s", rec.Package, rec.Label, rec.FilePath, rec.DeviceID)

	// Verify /data on APK Runner is 32 GiB.
	c, err := s.MobileControl(rec.DeviceID)
	if err != nil {
		t.Fatalf("MobileControl: %v", err)
	}
	dfOut, err := c.ADB.Shell(ctx, "df -h /data")
	if err != nil {
		t.Fatalf("df -h /data: %v", err)
	}
	t.Logf("APK Runner /data:\n%s", strings.TrimSpace(dfOut))
	if !strings.Contains(dfOut, "32G") && !strings.Contains(dfOut, "31G") {
		t.Fatalf("expected 32G /data on APK Runner, got:\n%s", dfOut)
	}

	// 3. Remove catalog entry (leaving app installed on VM) and verify ListUncatalogedAPKs detects it.
	if err := s.DeleteAPK(ctx, rec.Package, false); err != nil {
		t.Fatalf("DeleteAPK(uninstall=false): %v", err)
	}
	uncat, err := s.ListUncatalogedAPKs(ctx, "")
	if err != nil {
		t.Fatalf("ListUncatalogedAPKs: %v", err)
	}
	t.Logf("uncataloged apps detected: %+v", uncat)
	found := false
	for _, u := range uncat {
		if u.Package == rec.Package {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %s in ListUncatalogedAPKs, got %+v", rec.Package, uncat)
	}

	// 4. ImportInstalledAPK pulls the real installed APK from VM, inspects it, and re-adds to catalog.
	t.Log("step 4: pulling & importing installed APK via ImportInstalledAPK")
	imported, err := s.ImportInstalledAPK(ctx, APKImportInput{
		Package:        rec.Package,
		Orientation:    "portrait",
		CreateShortcut: false,
	})
	if err != nil {
		t.Fatalf("ImportInstalledAPK: %v", err)
	}
	t.Logf("imported from VM: pkg=%s label=%q ver=%s cache=%s", imported.Package, imported.Label, imported.VersionName, imported.FilePath)
	if imported.Package != rec.Package || imported.Label == "" || imported.FilePath == "" {
		t.Fatalf("unexpected imported record: %+v", imported)
	}

	// 5. OpenAPKStore deep-links Aurora Store to TikTok on the running APK Runner.
	t.Log("step 5: opening Aurora Store deep-linked to TikTok")
	if err := s.OpenAPKStore(ctx, APKStoreRequest{Query: "com.zhiliaoapp.musically"}); err != nil {
		t.Fatalf("OpenAPKStore: %v", err)
	}
	t.Log("OpenAPKStore succeeded")
}
