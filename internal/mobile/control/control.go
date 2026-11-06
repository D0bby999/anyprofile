// SPDX-License-Identifier: AGPL-3.0-or-later

// Package control drives a running Android device for an AI agent, over the
// adb that AnyProfile holds: the screen as an outline with refs, taps,
// swipes, typing, keys, screenshots, apps and files. It works the same on a
// macOS VM and an LDPlayer instance. Nothing an agent sends reaches the
// shell as code: text goes base64 through ADBKeyBoard, package names and
// keys are checked, files come from the uploads folder only.
package control

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image/jpeg"
	"image/png"
	"io"
	"math/rand/v2"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/adb"
)

// ADBKeyBoard (github.com/senzhk/ADBKeyBoard, GPL-2.0) is an input method
// that types what it receives in a broadcast, base64 encoded: Vietnamese,
// emoji, anything. It is installed on first use and set only while typing.
// Aurora Store (gitlab.com/AuroraOSS/AuroraStore, GPL-3.0, F-Droid build) is
// downloaded on demand when the user asks to install it.
// Updating the pins: docs/mobile-maintenance.md.
const (
	keyboardIME    = "com.android.adbkeyboard/.AdbIME"
	keyboardPkg    = "com.android.adbkeyboard"
	keyboardURL    = "https://github.com/senzhk/ADBKeyBoard/releases/download/v2.5-dev/keyboardservice-debug.apk"
	keyboardSHA256 = "41a8a0996d7397a2390d1ca16a75cb66c4a7bdaa89cf4e63600a4d3fb346fbbb"

	auroraPkg    = "com.aurora.store"
	auroraFile   = "aurora-store-4.8.4.apk"
	auroraURL    = "https://f-droid.org/repo/com.aurora.store_76.apk"
	auroraSHA256 = "fd9c75d90d0f4a7c132b9b4a5a2cf1992a45e03b8d8ff988b7dcfbc0db2c4d11"

	dumpPath  = "/data/local/tmp/anyprofile-ui.xml"
	pushDir   = "/sdcard/DCIM/AnyProfile"
	cmdTimout = 30 * time.Second
)

// Device is one running device. Its refs live until the next Snapshot.
type Device struct {
	ADB adb.Client
	// Runtime is where downloaded helper apps are kept.
	Runtime string
	// Locale is the BCP-47 language tag (e.g. "vi-VN") applied to newly installed apps.
	Locale string

	mu   sync.Mutex
	refs map[string]Rect

	shMu      sync.Mutex
	sh        *adb.Session
	metricsMu sync.Mutex
	metrics   mobile.Metrics

	wakeMu sync.Mutex
	wokeAt time.Time
}

func (d *Device) note(start time.Time) {
	d.metricsMu.Lock()
	defer d.metricsMu.Unlock()
	d.metrics.Actions++
	d.metrics.LastActionMS = time.Since(start).Milliseconds()
}

// Metrics reports this control handle's action timings.
func (d *Device) Metrics() mobile.Metrics {
	d.metricsMu.Lock()
	defer d.metricsMu.Unlock()
	return d.metrics
}

var errNoRef = apperr.New("mobile_ref", "that ref is not on the screen any more; take a new device_snapshot")

// wake turns the screen on before an action that needs it. A headless device's
// screen turns off after 15 s (the VM driver sets that); a sleeping Android
// still answers adb, but a screenshot would be black and a tap could land on
// the lock screen. The 10 s window keeps a burst of taps or the 400 ms preview
// loop from paying the keyevent every time; the screen timeout is 15 s, so the
// next burst after a pause wakes it again.
func (d *Device) wake(ctx context.Context) {
	d.wakeMu.Lock()
	if time.Since(d.wokeAt) < 10*time.Second {
		d.wakeMu.Unlock()
		return
	}
	d.wokeAt = time.Now()
	d.wakeMu.Unlock()
	_, _ = d.shell(ctx, "input keyevent KEYCODE_WAKEUP")
}

func (d *Device) shell(ctx context.Context, line string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, cmdTimout)
	defer cancel()
	d.shMu.Lock()
	defer d.shMu.Unlock()
	if d.sh == nil {
		if s, err := d.ADB.OpenShell(ctx); err == nil {
			d.sh = s
		}
	}
	if d.sh != nil {
		out, err := d.sh.Run(ctx, line)
		if err == nil || strings.HasPrefix(err.Error(), "shell exit ") {
			return out, err
		}
		d.sh.Close()
		d.sh = nil
	}
	return d.ADB.Shell(ctx, line)
}

// Close releases the persistent shell; the next action opens a new one.
func (d *Device) Close() {
	d.shMu.Lock()
	defer d.shMu.Unlock()
	if d.sh != nil {
		d.sh.Close()
		d.sh = nil
	}
}

// Snapshot is the screen as an outline. A screen that never settles (a video
// feed) cannot be dumped; the error says to use a screenshot instead.
func (d *Device) Snapshot(ctx context.Context) (string, error) {
	d.wake(ctx)
	start := time.Now()
	defer func() {
		d.metricsMu.Lock()
		d.metrics.Actions++
		d.metrics.SnapshotMS = time.Since(start).Milliseconds()
		d.metricsMu.Unlock()
	}()
	out, err := d.shell(ctx, "uiautomator dump "+dumpPath+" >/dev/null 2>&1 && cat "+dumpPath+"; rm -f "+dumpPath)
	if err != nil || !strings.Contains(out, "<hierarchy") {
		return "", apperr.New("mobile_snapshot", "the screen keeps moving (like a video feed) and cannot be read as text; use device_screenshot and tap by x and y")
	}
	outline, refs, err := Outline([]byte(out))
	if err != nil {
		return "", err
	}
	d.mu.Lock()
	d.refs = refs
	d.mu.Unlock()
	if outline == "" {
		outline = "(nothing to read on this screen; use device_screenshot)\n"
	}
	return outline, nil
}

func (d *Device) rect(ref string) (Rect, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	r, ok := d.refs[ref]
	if !ok {
		return Rect{}, errNoRef
	}
	return r, nil
}

// pointIn is a point near the middle of r, a few pixels off like a finger.
func pointIn(r Rect) (int, int) {
	x, y := r.Center()
	jx, jy := (r.X2-r.X1)/6, (r.Y2-r.Y1)/6
	if jx > 0 {
		x += rand.IntN(2*jx+1) - jx
	}
	if jy > 0 {
		y += rand.IntN(2*jy+1) - jy
	}
	return x, y
}

// Tap touches ref, or x and y when ref is "". A touch lasts 80-140 ms.
func (d *Device) Tap(ctx context.Context, ref string, x, y int) error {
	d.wake(ctx)
	start := time.Now()
	defer d.note(start)
	if ref != "" {
		r, err := d.rect(ref)
		if err != nil {
			return err
		}
		x, y = pointIn(r)
	}
	if x < 0 || y < 0 {
		return apperr.New("mobile_point", "x and y must be on the screen")
	}
	ms := 80 + rand.IntN(61)
	_, err := d.shell(ctx, fmt.Sprintf("input swipe %d %d %d %d %d", x, y, x, y, ms))
	return err
}

// screen is the display size, as wm reports it.
func (d *Device) screen(ctx context.Context) (int, int, error) {
	out, err := d.shell(ctx, "wm size")
	if err != nil {
		return 0, 0, err
	}
	// "Physical size: 1080x2400", maybe followed by "Override size: …".
	line := out
	if i := strings.LastIndex(out, "size: "); i >= 0 {
		line = out[i+6:]
	}
	w, h, ok := strings.Cut(strings.Fields(line)[0], "x")
	if !ok {
		return 0, 0, fmt.Errorf("wm size: %q", out)
	}
	wi, err1 := strconv.Atoi(w)
	hi, err2 := strconv.Atoi(h)
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("wm size: %q", out)
	}
	return wi, hi, nil
}

// Swipe moves a finger in a direction (up scrolls the content up, as on a
// feed) or between two points, in 250-600 ms.
func (d *Device) Swipe(ctx context.Context, direction string, x1, y1, x2, y2 int) error {
	d.wake(ctx)
	start := time.Now()
	defer d.note(start)
	if direction != "" {
		w, h, err := d.screen(ctx)
		if err != nil {
			return err
		}
		cx, cy := w/2+rand.IntN(w/10+1)-w/20, h/2
		switch direction {
		case "up":
			x1, y1, x2, y2 = cx, h*3/4, cx, h/4
		case "down":
			x1, y1, x2, y2 = cx, h/4, cx, h*3/4
		case "left":
			x1, y1, x2, y2 = w*4/5, cy, w/5, cy
		case "right":
			x1, y1, x2, y2 = w/5, cy, w*4/5, cy
		default:
			return apperr.New("mobile_direction", "direction is up, down, left or right")
		}
	}
	ms := 250 + rand.IntN(351)
	_, err := d.shell(ctx, fmt.Sprintf("input swipe %d %d %d %d %d", x1, y1, x2, y2, ms))
	return err
}

var keycodes = map[string]int{"back": 4, "home": 3, "recents": 187, "enter": 66, "delete": 67, "tab": 61,
	"search": 84, "menu": 82, "volume_up": 24, "volume_down": 25}

// PressKey presses one of keycodes' keys.
func (d *Device) PressKey(ctx context.Context, key string) error {
	d.wake(ctx)
	start := time.Now()
	defer d.note(start)
	code, ok := keycodes[strings.ToLower(key)]
	if !ok {
		return apperr.New("mobile_key", "key is back, home, recents, enter, delete, tab, search, menu, volume_up or volume_down")
	}
	_, err := d.shell(ctx, "input keyevent "+strconv.Itoa(code))
	return err
}

// Type taps ref first (when given), then types text with ADBKeyBoard, which
// takes it base64 encoded: no character can reach the shell. The input method
// the user had comes back afterwards.
func (d *Device) Type(ctx context.Context, ref, text string, submit bool) error {
	d.wake(ctx)
	start := time.Now()
	defer d.note(start)
	if ref != "" {
		if err := d.Tap(ctx, ref, 0, 0); err != nil {
			return err
		}
		time.Sleep(300 * time.Millisecond)
	}
	if err := d.ensureKeyboard(ctx); err != nil {
		return err
	}
	prev, _ := d.shell(ctx, "settings get secure default_input_method")
	if _, err := d.shell(ctx, "ime enable "+keyboardIME+" >/dev/null; ime set "+keyboardIME); err != nil {
		return err
	}
	defer func() {
		if prev != "" && prev != "null" && prev != keyboardIME && !strings.ContainsAny(prev, " ;'\"$`|&") {
			d.shell(context.Background(), "ime set "+prev)
		}
	}()
	// A broadcast sent before the keyboard is bound reaches no one and its text is lost:
	// the first switch after installing it takes over 300 ms.
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if out, _ := d.shell(ctx, "dumpsys input_method"); keyboardBound(out) {
			break
		}
	}
	// One chunk at a time, as a person types in bursts; base64 is [A-Za-z0-9+/=].
	runes := []rune(text)
	for i := 0; i < len(runes); {
		n := 4 + rand.IntN(9)
		if i+n > len(runes) {
			n = len(runes) - i
		}
		b64 := base64.StdEncoding.EncodeToString([]byte(string(runes[i : i+n])))
		if _, err := d.shell(ctx, "am broadcast -a ADB_INPUT_B64 --es msg "+b64+" >/dev/null"); err != nil {
			return err
		}
		i += n
		time.Sleep(time.Duration(25*n+rand.IntN(55*n)) * time.Millisecond)
	}
	if submit {
		return d.PressKey(ctx, "enter")
	}
	return nil
}

// keyboardBound reads dumpsys input_method: ADBKeyBoard is the current input
// method and the system has bound to it.
func keyboardBound(dump string) bool {
	return strings.Contains(dump, "mCurId="+keyboardIME) && strings.Contains(dump, "mBoundToMethod=true")
}

func (d *Device) ensureKeyboard(ctx context.Context) error {
	if out, _ := d.shell(ctx, "pm path "+keyboardPkg); strings.HasPrefix(out, "package:") {
		return nil
	}
	apk, err := adb.FetchAPK(ctx, d.Runtime, "adbkeyboard-2.5.apk", keyboardURL, keyboardSHA256)
	if err != nil {
		return err
	}
	ictx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	_, err = d.ADB.Run(ictx, "install", "-r", apk)
	return err
}

// Screenshot is the screen as a JPEG.
func (d *Device) Screenshot(ctx context.Context) ([]byte, error) {
	d.wake(ctx)
	ctx, cancel := context.WithTimeout(ctx, cmdTimout)
	defer cancel()
	raw, err := d.ADB.Output(ctx, "exec-out", "screencap", "-p")
	if err != nil {
		return nil, err
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("screenshot: %w", err)
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 75}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// Apps lists the apps the user installed.
func (d *Device) Apps(ctx context.Context) ([]string, error) {
	out, err := d.shell(ctx, "pm list packages -3")
	if err != nil {
		return nil, err
	}
	var pkgs []string
	for _, l := range strings.Split(out, "\n") {
		if p, ok := strings.CutPrefix(strings.TrimSpace(l), "package:"); ok && p != keyboardPkg {
			pkgs = append(pkgs, p)
		}
	}
	return pkgs, nil
}

// Launch opens an app's main screen.
func (d *Device) Launch(ctx context.Context, pkg string) error {
	d.wake(ctx)
	if !mobile.ValidPackage(pkg) {
		return apperr.New("mobile_package", "that is not an app package name; see device_list_apps")
	}
	out, err := d.shell(ctx, "monkey -p "+pkg+" -c android.intent.category.LAUNCHER 1")
	if err != nil || strings.Contains(out, "No activities found") {
		return apperr.Newf("mobile_package", "%s cannot be opened; see device_list_apps", pkg)
	}
	return nil
}

// OpenURL opens an http(s) address in the device's default browser. The
// address goes to the shell in single quotes, so it may hold none, nor
// control characters.
func (d *Device) OpenURL(ctx context.Context, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		strings.ContainsAny(raw, "'\\") || strings.IndexFunc(raw, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return apperr.New("mobile_url", "give an http or https address")
	}
	d.wake(ctx)
	_, err = d.shell(ctx, "am start -a android.intent.action.VIEW -d '"+raw+"' >/dev/null")
	return err
}

// Push copies a file from this computer into the device's photos folder
// (DCIM/AnyProfile), where gallery and upload pickers find it.
func (d *Device) Push(ctx context.Context, local string) (string, error) {
	name := filepath.Base(local)
	if strings.ContainsAny(name, "'\"$`\\;|&<>\n") {
		return "", apperr.New("mobile_file_name", "rename the file without quotes, $, ; or other special characters first")
	}
	remote := path.Join(pushDir, name)
	pctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if _, err := d.shell(ctx, "mkdir -p "+pushDir); err != nil {
		return "", err
	}
	if _, err := d.ADB.Run(pctx, "push", local, remote); err != nil {
		return "", err
	}
	if err := d.scan(ctx, remote); err != nil {
		return remote, fmt.Errorf("copied, but the gallery may not show it yet: %w", err)
	}
	return remote, nil
}

// scan tells the media store about a new file, so pickers list it.
func (d *Device) scan(ctx context.Context, remote string) error {
	_, err := d.shell(ctx, "content call --uri content://media --method scan_volume --arg external_primary >/dev/null 2>&1 || am broadcast -a android.intent.action.MEDIA_SCANNER_SCAN_FILE -d 'file://"+remote+"' >/dev/null")
	return err
}

// IsAppPackage reports whether path has an Android app extension (.apk or a
// split bundle: .apks, .xapk, .apkm).
func IsAppPackage(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".apk", ".apks", ".xapk", ".apkm":
		return true
	}
	return false
}

// Install installs an .apk or split bundle (.apks, .xapk, .apkm) from this computer.
func (d *Device) Install(ctx context.Context, pkgPath string) error {
	if !IsAppPackage(pkgPath) {
		return apperr.New("mobile_apk", "only .apk, .apks, .xapk and .apkm files install")
	}
	ictx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	args := []string{"install", "-r", pkgPath}
	if !strings.EqualFold(filepath.Ext(pkgPath), ".apk") {
		tmp, err := os.MkdirTemp("", "anyprofile-apks-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		splits, err := extractSplitAPKs(pkgPath, tmp)
		if err != nil {
			return err
		}
		args = append([]string{"install-multiple", "-r"}, splits...)
	}
	out, err := d.ADB.Run(ictx, args...)
	if err != nil {
		return err
	}
	if !strings.Contains(out, "Success") {
		return errors.New(out)
	}
	d.syncAppLocales(ctx)
	return nil
}

func (d *Device) syncAppLocales(ctx context.Context) {
	if !mobile.ValidLocale(d.Locale) {
		return
	}
	pkgs, err := d.Apps(ctx)
	if err != nil || len(pkgs) == 0 {
		return
	}
	var cmds []string
	for _, p := range pkgs {
		if mobile.ValidPackage(p) {
			cmds = append(cmds, "cmd locale set-app-locales "+p+" --locales "+d.Locale)
		}
	}
	if len(cmds) > 0 {
		_, _ = d.shell(ctx, strings.Join(cmds, "; "))
	}
}

// InstallAurora downloads Aurora Store from F-Droid (cached in Runtime and
// checked by SHA-256) and installs and opens it on the device.
func (d *Device) InstallAurora(ctx context.Context) error {
	apk, err := adb.FetchAPK(ctx, d.Runtime, auroraFile, auroraURL, auroraSHA256)
	if err != nil {
		return err
	}
	if err := d.Install(ctx, apk); err != nil {
		return err
	}
	_ = d.Launch(ctx, auroraPkg)
	return nil
}

func extractSplitAPKs(bundlePath, dir string) ([]string, error) {
	zr, err := zip.OpenReader(bundlePath)
	if err != nil {
		return nil, apperr.New("mobile_apk", "corrupted or encrypted bundle; use an unencrypted .apk, .apks or .xapk file")
	}
	defer zr.Close()
	const maxSplits = 64
	var rem int64 = 2 << 30 // 2 GiB cap across extracted splits
	var out []string
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || strings.HasPrefix(f.Name, "__MACOSX/") || strings.HasPrefix(filepath.Base(f.Name), ".") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(f.Name))
		if ext == ".obb" {
			return nil, apperr.New("mobile_apk", "bundles with .obb data files are not supported")
		}
		if ext != ".apk" {
			continue
		}
		if len(out) >= maxSplits {
			return nil, apperr.New("mobile_apk", "bundle has too many split APKs")
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		dst := filepath.Join(dir, fmt.Sprintf("split-%02d.apk", len(out)))
		wf, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			rc.Close()
			return nil, err
		}
		n, copyErr := io.Copy(wf, io.LimitReader(rc, rem+1))
		rc.Close()
		closeErr := wf.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		rem -= n
		if rem < 0 {
			return nil, apperr.New("mobile_apk", "bundle is larger than 2 GB")
		}
		out = append(out, dst)
	}
	if len(out) == 0 {
		return nil, apperr.New("mobile_apk", "bundle contains no .apk files")
	}
	return out, nil
}

// WaitFor waits until text shows on the screen (or is gone), at most seconds.
func (d *Device) WaitFor(ctx context.Context, text string, gone bool, seconds int) (bool, error) {
	if seconds <= 0 {
		seconds = 10
	}
	deadline := time.Now().Add(time.Duration(min(seconds, 60)) * time.Second)
	for {
		outline, err := d.Snapshot(ctx)
		if err == nil && strings.Contains(strings.ToLower(outline), strings.ToLower(text)) != gone {
			return true, nil
		}
		if time.Now().After(deadline) {
			return false, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
