// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package avo

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/adb"
	"github.com/D0bby999/anyprofile/internal/mobile/control"
	"github.com/D0bby999/anyprofile/internal/mobile/vm"
)

const (
	gappsReadyStamp = "cuttlefish-mtg-16.0-v3\n"
	cleanReadyStamp = "cuttlefish-clean-16.0-v2\n"
)

// instanceDisks are the per-instance mutable Cuttlefish disk files backed up,
// restored, and cloned from the shared base templates.
var instanceDisks = []string{
	"super.img",
	"userdata.img",
	"metadata.img",
	"misc.img",
	"vbmeta.img",
	"frp.img",
}

func gmsBaseDir(rt Runtime) string   { return filepath.Join(rt.ImageDir, "gms") }
func cleanBaseDir(rt Runtime) string { return filepath.Join(rt.ImageDir, "clean") }

func baseReady(dir, stamp string) (string, bool) {
	b, err := os.ReadFile(filepath.Join(dir, "READY"))
	if err != nil || string(b) != stamp {
		return "", false
	}
	for _, f := range instanceDisks {
		if !exists(filepath.Join(dir, f)) {
			return "", false
		}
	}
	return dir, true
}

func gmsBaseReady(rt Runtime) (string, bool)   { return baseReady(gmsBaseDir(rt), gappsReadyStamp) }
func cleanBaseReady(rt Runtime) (string, bool) { return baseReady(cleanBaseDir(rt), cleanReadyStamp) }

// bakeAurora pre-installs Aurora Store into the shared base disk so every new
// device can install apps immediately without signing in to Google Play, and
// configures a clean Launcher3 home screen with Play Store & Aurora Store.
func bakeAurora(ctx context.Context, log *slog.Logger, a adb.Client, cacheDir string) {
	c, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	apk, err := control.FetchAurora(c, cacheDir)
	if err != nil {
		log.Warn("avo base: fetch aurora store", "err", err)
		return
	}
	if _, err := a.Run(c, "install", "-r", "-g", apk); err != nil {
		log.Warn("avo base: install aurora store", "err", err)
		return
	}
	_, _ = a.Shell(c, control.AuroraGrantScript)
	configureLauncherHome(c, a, "", log)
}

const launcherHomeScript = `for pkg in \
  com.android.threadnetwork.demoapp \
  com.android.privatespace \
  com.android.quicksearchbox \
  com.android.dialer \
  com.android.messaging \
  com.android.contacts \
  com.android.calendar \
  com.android.deskclock; do
  pm disable-user --user 0 "$pkg" >/dev/null 2>&1 || true
done
pm enable org.chromium.webview_shell >/dev/null 2>&1 || true
BROWSER_PKG="org.chromium.webview_shell"
BROWSER_ACT="org.chromium.webview_shell/.WebViewBrowserActivity"
if pm path com.android.chrome >/dev/null 2>&1; then
  BROWSER_PKG="com.android.chrome"
  BROWSER_ACT="com.android.chrome/com.google.android.apps.chrome.Main"
fi
DB=/data/data/com.android.launcher3/databases/launcher.db
if [ -f "$DB" ] && command -v sqlite3 >/dev/null 2>&1; then
  sqlite3 "$DB" "
    DELETE FROM favorites;
    INSERT INTO favorites (_id, title, intent, container, screen, cellX, cellY, spanX, spanY, itemType, profileId, rank) VALUES
      (1, \"Play Store\", \"#Intent;action=android.intent.action.MAIN;category=android.intent.category.LAUNCHER;launchFlags=0x10200000;package=com.android.vending;component=com.android.vending/.AssetBrowserActivity;end\", -100, 0, 0, 0, 1, 1, 0, 0, 0),
      (2, \"Aurora Store\", \"#Intent;action=android.intent.action.MAIN;category=android.intent.category.LAUNCHER;launchFlags=0x10200000;package=com.aurora.store;component=com.aurora.store/.ComposeActivity;end\", -100, 0, 1, 0, 1, 1, 0, 0, 0),
      (3, \"Chrome\", \"#Intent;action=android.intent.action.MAIN;category=android.intent.category.LAUNCHER;launchFlags=0x10200000;package=${BROWSER_PKG};component=${BROWSER_ACT};end\", -101, 0, 0, 0, 1, 1, 0, 0, 0),
      (4, \"Files\", \"#Intent;action=android.intent.action.MAIN;category=android.intent.category.LAUNCHER;launchFlags=0x10200000;package=com.android.documentsui;component=com.android.documentsui/.LauncherActivity;end\", -101, 1, 1, 0, 1, 1, 0, 0, 0),
      (5, \"Gallery\", \"#Intent;action=android.intent.action.MAIN;category=android.intent.category.LAUNCHER;launchFlags=0x10200000;package=com.android.gallery3d;component=com.android.gallery3d/.app.GalleryActivity;end\", -101, 2, 2, 0, 1, 1, 0, 0, 0),
      (6, \"Camera\", \"#Intent;action=android.intent.action.MAIN;category=android.intent.category.LAUNCHER;launchFlags=0x10200000;package=com.android.camera2;component=com.android.camera2/com.android.camera.CameraLauncher;end\", -101, 3, 3, 0, 1, 1, 0, 0, 0),
      (7, \"Settings\", \"#Intent;action=android.intent.action.MAIN;category=android.intent.category.LAUNCHER;launchFlags=0x10200000;package=com.android.settings;component=com.android.settings/.Settings;end\", -101, 4, 4, 0, 1, 1, 0, 0, 0);
  "
  am force-stop com.android.launcher3 >/dev/null 2>&1 || true
  input keyevent KEYCODE_HOME >/dev/null 2>&1 || true
fi`

const homeLayoutStamp = "v2\n"

func configureLauncherHome(ctx context.Context, a adb.Client, instDir string, log *slog.Logger) {
	c, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := launcherHomeScript
	if _, err := a.Shell(c, "su 0 sh -c '"+strings.ReplaceAll(cmd, "'", "'\\''")+"'"); err != nil {
		if _, err2 := a.Shell(c, cmd); err2 != nil {
			log.Warn("avo launcher home layout", "err", err)
			return
		}
	}
	if instDir != "" {
		_ = os.WriteFile(filepath.Join(instDir, ".home_layout_ready"), []byte(homeLayoutStamp), 0o600)
	}
}

func homeLayoutReady(instDir string) bool {
	if instDir == "" {
		return false
	}
	b, err := os.ReadFile(filepath.Join(instDir, ".home_layout_ready"))
	return err == nil && string(b) == homeLayoutStamp
}

func cloneBaseDisks(srcDir, dstDir string) error {
	if err := os.MkdirAll(dstDir, 0o700); err != nil {
		return err
	}
	for _, f := range instanceDisks {
		if err := vm.CloneOrCopy(filepath.Join(srcDir, f), filepath.Join(dstDir, f)); err != nil {
			return err
		}
	}
	if exists(filepath.Join(srcDir, ".gpu_layers_ready")) {
		_ = os.WriteFile(filepath.Join(dstDir, ".gpu_layers_ready"), []byte("1\n"), 0o600)
	}
	return nil
}

func cloneGMSDisks(rt Runtime, dstDir string) error {
	srcDir, ok := gmsBaseReady(rt)
	if !ok {
		return errors.New("GMS base is not prepared")
	}
	return cloneBaseDisks(srcDir, dstDir)
}

func cloneCleanDisks(rt Runtime, dstDir string) error {
	srcDir, ok := cleanBaseReady(rt)
	if !ok {
		return errors.New("clean base is not prepared")
	}
	return cloneBaseDisks(srcDir, dstDir)
}

// PrepareCleanBase boots Cuttlefish once without GMS so userdata.img is pre-formatted,
// GPU layers are baked in, and VirtWifi is pre-enabled; subsequent non-GMS devices
// clone in <50ms via APFS clonefile instead of formatting 8 GB f2fs on first boot.
func (d *Driver) PrepareCleanBase(ctx context.Context, rt Runtime) (string, error) {
	if dir, ok := cleanBaseReady(rt); ok {
		return dir, nil
	}
	d.gmsMu.Lock()
	defer d.gmsMu.Unlock()
	if dir, ok := cleanBaseReady(rt); ok {
		return dir, nil
	}

	d.log.Info("preparing AnyMobile Avo clean base image (one-time setup)")
	outDir := cleanBaseDir(rt)
	_ = os.RemoveAll(outDir)
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}
	workDir, err := os.MkdirTemp(outDir, "build-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(workDir)

	instDir := filepath.Join(workDir, "inst")
	if err := os.MkdirAll(instDir, 0o700); err != nil {
		return "", err
	}
	dummy := mobile.Device{
		ID:      "clean-build",
		Name:    "clean-build",
		CPUs:    4,
		RAMMB:   4096,
		ScreenW: 780,
		ScreenH: 1688,
		Density: 320,
	}
	p, err := d.launchVMM(ctx, rt, dummy, instDir, vm.OfflineDialer(), 5*time.Minute, nil)
	if err != nil {
		return "", fmt.Errorf("boot cuttlefish for clean base: %w", err)
	}
	stopBuild := func() {
		sctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = d.shutdownProc(sctx, p)
	}
	ok := false
	defer func() {
		if !ok {
			stopBuild()
		}
	}()

	ensureGPULayers(ctx, rt, p.adb, outDir, d.log)
	bakeAurora(ctx, d.log, p.adb, d.qemuRuntime)
	_, _ = p.adb.Shell(ctx, "locksettings set-disabled true >/dev/null 2>&1 || true; settings put global captive_portal_mode 0; cmd wifi set-wifi-enabled enabled; sync")
	stopBuild()

	for _, f := range instanceDisks {
		src := filepath.Join(instDir, f)
		if !exists(src) {
			return "", fmt.Errorf("missing built clean disk %s", f)
		}
		if err := os.Rename(src, filepath.Join(outDir, f)); err != nil {
			if err := vm.CopyFile(src, filepath.Join(outDir, f)); err != nil {
				return "", err
			}
		}
	}
	if err := os.WriteFile(filepath.Join(outDir, "READY"), []byte(cleanReadyStamp), 0o600); err != nil {
		return "", err
	}
	ok = true
	d.log.Info("AnyMobile Avo clean base image ready", "dir", outDir)
	return outDir, nil
}

// PrepareGMSBase builds the shared MindTheGapps-enabled Cuttlefish disk set once
// under image/<version>/gms/ using avbctl disable-verity + adb remount so every
// subsequent WithGMS device clones in <1s via APFS clonefile.
func (d *Driver) PrepareGMSBase(ctx context.Context, rt Runtime) (string, error) {
	if dir, ok := gmsBaseReady(rt); ok {
		return dir, nil
	}
	d.gmsMu.Lock()
	defer d.gmsMu.Unlock()
	if dir, ok := gmsBaseReady(rt); ok {
		return dir, nil
	}

	cacheDir := filepath.Join(d.qemuRuntime, "cache")
	d.log.Info("preparing AnyMobile Avo GMS base image (one-time setup)")
	zipPath, err := vm.FetchMindTheGapps(ctx, cacheDir)
	if err != nil {
		return "", fmt.Errorf("fetch MindTheGapps: %w", err)
	}

	outDir := gmsBaseDir(rt)
	_ = os.RemoveAll(outDir)
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}
	workDir, err := os.MkdirTemp(outDir, "build-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(workDir)

	stageDir := filepath.Join(workDir, "gapps")
	if err := extractMindTheGapps(zipPath, stageDir); err != nil {
		return "", fmt.Errorf("extract MindTheGapps: %w", err)
	}

	instDir := filepath.Join(workDir, "inst")
	if err := os.MkdirAll(instDir, 0o700); err != nil {
		return "", err
	}
	dummy := mobile.Device{
		ID:      "gms-build",
		Name:    "gms-build",
		CPUs:    4,
		RAMMB:   4096,
		ScreenW: 780,
		ScreenH: 1688,
		Density: 320,
	}
	p, err := d.launchVMM(ctx, rt, dummy, instDir, vm.OfflineDialer(), 5*time.Minute, nil)
	if err != nil {
		return "", fmt.Errorf("boot cuttlefish for GMS: %w", err)
	}
	stopBuild := func() {
		sctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = d.shutdownProc(sctx, p)
	}
	ok := false
	defer func() {
		if !ok {
			stopBuild()
		}
	}()

	if err := installGappsOnRunning(ctx, d.log, p, stageDir, func(timeout time.Duration) error {
		return d.rebootAndWait(ctx, p, timeout)
	}); err != nil {
		return "", err
	}
	ensureGPULayers(ctx, rt, p.adb, outDir, d.log)
	bakeAurora(ctx, d.log, p.adb, d.qemuRuntime)
	_, _ = p.adb.Shell(ctx, "locksettings set-disabled true >/dev/null 2>&1 || true; settings put global captive_portal_mode 0; cmd wifi set-wifi-enabled enabled; sync")
	// Cleanly power off before copying overlay disks.
	stopBuild()

	for _, f := range instanceDisks {
		src := filepath.Join(instDir, f)
		if !exists(src) {
			return "", fmt.Errorf("missing built GMS disk %s", f)
		}
		if err := os.Rename(src, filepath.Join(outDir, f)); err != nil {
			if err := vm.CopyFile(src, filepath.Join(outDir, f)); err != nil {
				return "", err
			}
		}
	}
	if err := os.WriteFile(filepath.Join(outDir, "READY"), []byte(gappsReadyStamp), 0o600); err != nil {
		return "", err
	}
	ok = true
	d.log.Info("AnyMobile Avo GMS base image ready", "dir", outDir)
	return outDir, nil
}

func (d *Driver) rebootAndWait(ctx context.Context, p *proc, timeout time.Duration) error {
	c, cancel := context.WithTimeout(ctx, 10*time.Second)
	_, _ = p.adb.Shell(c, "svc power stayon false; sync; reboot")
	cancel()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.exited:
		return errors.New("cuttlefish exited unexpectedly during GMS reboot")
	case <-time.After(4 * time.Second):
	}
	wctx, wcancel := context.WithTimeout(ctx, timeout)
	defer wcancel()
	p.adb.Disconnect()
	return waitBootClean(wctx, p.adb, p.exited)
}

func installGappsOnRunning(ctx context.Context, log *slog.Logger, p *proc, stageDir string, reboot func(time.Duration) error) error {
	log.Info("gms step: adb root")
	if err := adbRootAndWait(ctx, p.adb, p.exited); err != nil {
		return err
	}
	out, _ := p.adb.Shell(ctx, "avbctl get-verity")
	if strings.Contains(strings.ToLower(out), "enabled") {
		log.Info("gms step: disabling verity")
		if _, err := p.adb.Shell(ctx, "avbctl disable-verity"); err != nil {
			return fmt.Errorf("avbctl disable-verity: %w", err)
		}
		if err := reboot(4 * time.Minute); err != nil {
			return fmt.Errorf("reboot after disable-verity: %w", err)
		}
		if err := adbRootAndWait(ctx, p.adb, p.exited); err != nil {
			return err
		}
	}

	log.Info("gms step: adb remount")
	remOut, err := p.adb.Run(ctx, "remount")
	if err != nil {
		return fmt.Errorf("adb remount: %w (%s)", err, remOut)
	}
	mounts, _ := p.adb.Shell(ctx, "cat /proc/mounts")
	needsReboot := strings.Contains(strings.ToLower(remOut), "reboot") ||
		!strings.Contains(mounts, "overlay /product overlay")
	if needsReboot {
		log.Info("gms step: rebooting for overlayfs")
		if err := reboot(4 * time.Minute); err != nil {
			return fmt.Errorf("reboot for overlayfs remount: %w", err)
		}
		if err := adbRootAndWait(ctx, p.adb, p.exited); err != nil {
			return err
		}
		if rem2, err := p.adb.Run(ctx, "remount"); err != nil {
			return fmt.Errorf("adb remount (post-reboot): %w (%s)", err, rem2)
		}
	}

	log.Info("gms step: pushing MindTheGapps")
	_, _ = p.adb.Shell(ctx, "rm -rf /data/local/tmp/avo-gapps && mkdir -p /data/local/tmp/avo-gapps")
	for _, part := range []string{"product", "system_ext"} {
		localPart := filepath.Join(stageDir, part)
		if !exists(localPart) {
			continue
		}
		if out, err := p.adb.Run(ctx, "push", localPart, "/data/local/tmp/avo-gapps/"); err != nil {
			return fmt.Errorf("push %s: %w (%s)", part, err, out)
		}
	}

	log.Info("gms step: committing files")
	copyScript := `set -e
S=/data/local/tmp/avo-gapps
find "$S" -type d -exec chmod 0755 {} +
find "$S" -type f -exec chmod 0644 {} +
chown -R root:root "$S"
mount -o remount,async /mnt/scratch >/dev/null 2>&1 || true
for P in product system_ext; do
  if [ -d "$S/$P" ]; then
    cp -a "$S/$P/." "/$P/"
    (cd "$S" && find "$P" -mindepth 1 | sed 's|^|/|' | xargs -n 64 restorecon >/dev/null 2>&1 || true)
  fi
done
rm -rf "$S"
sync
mount -o remount,sync /mnt/scratch >/dev/null 2>&1 || true`
	cctx, ccancel := context.WithTimeout(ctx, 2*time.Minute)
	defer ccancel()
	if out, err := p.adb.Shell(cctx, copyScript); err != nil {
		return fmt.Errorf("commit MindTheGapps: %w (%s)", err, out)
	}
	if _, err := p.adb.Shell(ctx, "test -f /product/priv-app/Phonesky/Phonesky.apk"); err != nil {
		return errors.New("Phonesky.apk missing after MindTheGapps install")
	}
	return nil
}

func adbRootAndWait(ctx context.Context, a adb.Client, exited <-chan struct{}) error {
	rctx, rcancel := context.WithTimeout(ctx, 15*time.Second)
	_, _ = a.Run(rctx, "root")
	rcancel()

	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-exited:
			return errors.New("cuttlefish exited while restarting adbd as root")
		case <-time.After(800 * time.Millisecond):
		}
		c, cancel := context.WithTimeout(ctx, 3*time.Second)
		a.Connect(c)
		idOut, err := a.Shell(c, "id")
		cancel()
		if err == nil && strings.Contains(idOut, "uid=0") {
			return nil
		}
	}
	return errors.New("timed out waiting for adb root")
}

func extractMindTheGapps(zipPath, dstDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	skipSubstrings := []string{
		"/Velvet/",
		"/VelvetTitan/",
		"/SetupWizard/",
		"/GoogleRestore/",
		"/GmsSetupWizardOverlay",
		"/AndroidAutoStub/",
		"/FamilyLinkParentalControls/",
		"/talkback/",
		"/SpeechServicesByGoogle/",
	}
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		var rel string
		switch {
		case strings.HasPrefix(f.Name, "system/product/"):
			rel = strings.TrimPrefix(f.Name, "system/")
		case strings.HasPrefix(f.Name, "system/system_ext/"):
			rel = strings.TrimPrefix(f.Name, "system/")
		default:
			continue
		}
		skip := false
		for _, s := range skipSubstrings {
			if strings.Contains(rel, s) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		target := filepath.Join(dstDir, filepath.FromSlash(rel))
		if !strings.HasPrefix(target, filepath.Clean(dstDir)+string(os.PathSeparator)) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		w, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(w, rc)
		wErr := w.Close()
		rc.Close()
		if err != nil {
			return err
		}
		if wErr != nil {
			return wErr
		}
	}
	return nil
}
