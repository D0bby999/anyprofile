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

func TestAPKDirectStoreInstallAndDeleteNoResurrect(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	t.Cleanup(func() {
		if id, ok := s.apkHostID(""); ok && s.mobileRunning(id) {
			sctx, scancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer scancel()
			_ = s.StopMobileDevice(sctx, id)
		}
	})

	// 1. Direct download & install of an Aptoide split-APK bundle (Shopee VN: base + arm64 + density splits).
	t.Log("step 1: InstallStoreAPK(com.shopee.vn) — real split-APK download & install")
	shopee, err := s.InstallStoreAPK(ctx, APKStoreInstallInput{
		Package:        "com.shopee.vn",
		Orientation:    "portrait",
		CreateShortcut: false,
		LaunchAfter:    false,
	})
	if err != nil {
		t.Fatalf("InstallStoreAPK(com.shopee.vn): %v", err)
	}
	t.Logf("installed split-APK: pkg=%s label=%q ver=%s abis=%v file=%s", shopee.Package, shopee.Label, shopee.VersionName, shopee.ABIs, shopee.FilePath)
	if shopee.Package != "com.shopee.vn" || shopee.FilePath != "" {
		t.Fatalf("expected store install without leftover cached bundle for shopee, got %+v", shopee)
	}

	// 2. Direct download & install of an F-Droid / Aptoide single APK (NewPipe).
	t.Log("step 2: InstallStoreAPK(org.schabi.newpipe)")
	newpipe, err := s.InstallStoreAPK(ctx, APKStoreInstallInput{
		Package:        "org.schabi.newpipe",
		Orientation:    "portrait",
		CreateShortcut: false,
		LaunchAfter:    false,
	})
	if err != nil {
		t.Fatalf("InstallStoreAPK(org.schabi.newpipe): %v", err)
	}
	t.Logf("installed single APK: pkg=%s label=%q ver=%s", newpipe.Package, newpipe.Label, newpipe.VersionName)

	// 3. Pause APK Runner, then delete NewPipe while PAUSED (verifies resume -> uninstall -> re-pause).
	hostID, ok := s.apkHostID("")
	if !ok {
		t.Fatal("missing apkHostID")
	}
	if err := drv.Pause(hostID); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	t.Log("step 3: DeleteAPK(org.schabi.newpipe, uninstall=true) while APK Runner is PAUSED")
	if err := s.DeleteAPK(ctx, "org.schabi.newpipe", true); err != nil {
		t.Fatalf("DeleteAPK(newpipe): %v", err)
	}
	if pending := s.pendingAPKUninstalls(); len(pending) != 0 {
		t.Fatalf("expected paused uninstall to succeed immediately, pending=%v", pending)
	}

	// 4. Stop APK Runner completely, then delete Shopee while STOPPED (queues pending uninstall).
	t.Log("step 4: stopping APK Runner and DeleteAPK(com.shopee.vn, uninstall=true) while STOPPED")
	if err := s.StopMobileDevice(ctx, hostID); err != nil {
		t.Fatalf("StopMobileDevice: %v", err)
	}
	if err := s.DeleteAPK(ctx, "com.shopee.vn", true); err != nil {
		t.Fatalf("DeleteAPK(shopee): %v", err)
	}
	if pending := s.pendingAPKUninstalls(); len(pending) != 1 || pending[0] != "com.shopee.vn" {
		t.Fatalf("expected com.shopee.vn queued for pending uninstall, got %v", pending)
	}

	// 5. Install a brand new app (Telegram) via InstallStoreAPK with LaunchAfter=false (headless launch via LaunchAPKHeadless)
	//    and verify BOTH deleted apps are gone on the VM, never reappear in ListUncatalogedAPKs, and Telegram launches cleanly.
	t.Log("step 5: InstallStoreAPK(org.telegram.messenger) + LaunchAPKHeadless")
	tg, err := s.InstallStoreAPK(ctx, APKStoreInstallInput{
		Package:        "org.telegram.messenger",
		Orientation:    "portrait",
		CreateShortcut: false,
		LaunchAfter:    false,
	})
	if err != nil {
		t.Fatalf("InstallStoreAPK(org.telegram.messenger): %v", err)
	}
	if err := s.LaunchAPKHeadless(ctx, tg.Package); err != nil {
		t.Fatalf("LaunchAPKHeadless(telegram): %v", err)
	}

	if pending := s.pendingAPKUninstalls(); len(pending) != 0 {
		t.Fatalf("expected pending uninstalls flushed on boot, still pending: %v", pending)
	}
	c, err := s.MobileControl(hostID)
	if err != nil {
		t.Fatalf("MobileControl: %v", err)
	}
	pkgsOut, err := c.ADB.Shell(ctx, "pm list packages -3")
	if err != nil {
		t.Fatalf("pm list packages -3: %v", err)
	}
	t.Logf("installed 3rd-party packages on VM:\n%s", strings.TrimSpace(pkgsOut))
	if strings.Contains(pkgsOut, "org.schabi.newpipe") || strings.Contains(pkgsOut, "com.shopee.vn") {
		t.Fatalf("deleted packages still present on VM:\n%s", pkgsOut)
	}
	if !strings.Contains(pkgsOut, tg.Package) {
		t.Fatalf("expected %s on VM, got:\n%s", tg.Package, pkgsOut)
	}

	uncat, err := s.ListUncatalogedAPKs(ctx, "")
	if err != nil {
		t.Fatalf("ListUncatalogedAPKs: %v", err)
	}
	if len(uncat) != 0 {
		t.Fatalf("expected 0 uncataloged/resurrected APKs, got %+v", uncat)
	}
	catalog, err := s.ListAPKs()
	if err != nil {
		t.Fatalf("ListAPKs: %v", err)
	}
	if len(catalog) != 1 || catalog[0].Package != tg.Package {
		t.Fatalf("expected only %s in catalog, got %+v", tg.Package, catalog)
	}

	// Verify Telegram actually launched to foreground.
	time.Sleep(2 * time.Second)
	focusOut, _ := c.ADB.Shell(ctx, "dumpsys window | grep -E 'mCurrentFocus|mFocusedApp'")
	t.Logf("foreground window after LaunchAPKHeadless:\n%s", strings.TrimSpace(focusOut))
	if !strings.Contains(focusOut, tg.Package) {
		t.Fatalf("expected %s in foreground, got:\n%s", tg.Package, focusOut)
	}
}

func TestAPKTikTokLiveAndAuroraFallback(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	t.Cleanup(func() {
		if id, ok := s.apkHostID(""); ok && s.mobileRunning(id) {
			sctx, scancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer scancel()
			_ = s.StopMobileDevice(sctx, id)
		}
	})

	// 1. Direct download & install of TikTok (4-part split APK ~199 MB) + launch headless.
	t0 := time.Now()
	t.Log("step 1: InstallStoreAPK(com.zhiliaoapp.musically)")
	tt, err := s.InstallStoreAPK(ctx, APKStoreInstallInput{
		Package:        "com.zhiliaoapp.musically",
		Orientation:    "portrait",
		CreateShortcut: false,
		LaunchAfter:    false,
	})
	if err != nil {
		t.Fatalf("InstallStoreAPK(TikTok): %v", err)
	}
	t.Logf("TikTok installed in %s: pkg=%s label=%q ver=%s activity=%q abis=%v",
		time.Since(t0).Round(time.Second), tt.Package, tt.Label, tt.VersionName, tt.LaunchActivity, tt.ABIs)

	if err := s.LaunchAPKHeadless(ctx, tt.Package); err != nil {
		t.Fatalf("LaunchAPKHeadless(TikTok): %v", err)
	}
	hostID, _ := s.apkHostID("")
	c, err := s.MobileControl(hostID)
	if err != nil {
		t.Fatalf("MobileControl: %v", err)
	}
	time.Sleep(6 * time.Second)
	focusOut, _ := c.ADB.Shell(ctx, "dumpsys window | grep -E 'mCurrentFocus|mFocusedApp'")
	t.Logf("TikTok foreground:\n%s", strings.TrimSpace(focusOut))
	if !strings.Contains(focusOut, "com.zhiliaoapp.musically") {
		pidOut, _ := c.ADB.Shell(ctx, "logcat -d | grep -E 'FATAL EXCEPTION|Process: com.zhiliaoapp.musically|aoapp.musically|ClassLoader|DexPathList|dalvikvm|art.*musically|ziparchive' | tail -n 60")
		t.Fatalf("TikTok did not stay in foreground:\n%s\nart/dex log:\n%s", focusOut, pidOut)
	}
	// Capture real screenshot of TikTok running on APK Runner.
	if png, err := c.Screenshot(ctx); err == nil && len(png) > 0 {
		outPath := "/Users/dobby/.gemini/antigravity/brain/7ca0e5b0-3741-4196-b823-4a04bc3dd94e/scratch/live_apk_tiktok.png"
		_ = os.WriteFile(outPath, png, 0o644)
		t.Logf("saved TikTok screenshot: %s (%d bytes)", outPath, len(png))
	}

	// 2. Verify Aurora Store fallback on fresh VM does NOT crash with HeaderProvider NPE.
	t.Log("step 2: OpenAPKStore fallback on fresh VM (must open onboarding cleanly, 0 NPE)")
	if err := s.OpenAPKStore(ctx, APKStoreRequest{Query: "com.ss.android.ugc.trill"}); err != nil {
		t.Fatalf("OpenAPKStore: %v", err)
	}
	time.Sleep(3 * time.Second)
	auroraLog, _ := c.ADB.Shell(ctx, "logcat -d | grep -E 'HeaderProvider|NullPointerException|com.aurora.store' | tail -n 20")
	if strings.Contains(auroraLog, "HeaderProvider.getDefaultHeaders") {
		t.Fatalf("Aurora Store still threw HeaderProvider NPE:\n%s", auroraLog)
	}
	t.Log("Aurora Store opened cleanly with 0 HeaderProvider NPE")
}
