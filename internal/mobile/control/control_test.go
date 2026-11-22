// SPDX-License-Identifier: AGPL-3.0-or-later

package control

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/mobile/adb"
)

// The testdata are screens of LineageOS 23.2 dumped on a real VM (2026-10-23).
func outline(t *testing.T, name string) (string, map[string]Rect) {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name + ".xml")
	if err != nil {
		t.Fatal(err)
	}
	o, refs, err := Outline(b)
	if err != nil {
		t.Fatal(err)
	}
	return o, refs
}

func TestOutlineLauncher(t *testing.T) {
	o, refs := outline(t, "launcher")
	if !strings.Contains(o, `- TextView "Browser"`) || strings.Contains(o, `"Browser" (Browser)`) {
		t.Fatalf("outline:\n%s", o)
	}
	m := regexp.MustCompile(`"Browser"[^\n]*\[ref=(m\d+)\]`).FindStringSubmatch(o)
	if m == nil {
		t.Fatalf("Browser has no ref:\n%s", o)
	}
	r := refs[m[1]]
	if r.Empty() || r.X2 > 1080 || r.Y2 > 2400 {
		t.Fatalf("bounds %+v", r)
	}
	// Every ref in the text has bounds, and the other way round.
	if got := len(regexp.MustCompile(`\[ref=m\d+\]`).FindAllString(o, -1)); got != len(refs) {
		t.Fatalf("%d refs in the outline, %d with bounds", got, len(refs))
	}
}

func TestOutlineBrowserAndTyped(t *testing.T) {
	o, _ := outline(t, "browser")
	if !regexp.MustCompile(`\[ref=m\d+\][^\n]*editable`).MatchString(o) {
		t.Fatalf("no editable field:\n%s", o)
	}
	typed, _ := outline(t, "typed")
	if !strings.Contains(typed, "Xin chào") || !strings.Contains(typed, "tiếng Việt") {
		t.Fatalf("typed text missing:\n%s", typed)
	}
}

func TestOutlineLockScreenAndErrors(t *testing.T) {
	o, _ := outline(t, "lockscreen")
	if !strings.Contains(o, "Unlock for all features") {
		t.Fatalf("%s", o)
	}
	if _, _, err := Outline([]byte("ERROR: could not get idle state.")); err == nil {
		t.Fatal("parsed a failed dump")
	}
	pw := []byte(`<hierarchy><node class="android.widget.EditText" text="hunter2" password="true" enabled="true" clickable="true" bounds="[0,0][10,10]"/></hierarchy>`)
	if o, _, _ := Outline(pw); strings.Contains(o, "hunter2") || !strings.Contains(o, "password") {
		t.Fatalf("password field: %q", o)
	}
}

func TestFormatADBInstallError(t *testing.T) {
	spaceErr := formatADBInstallError("NewPipe.apk", "adb install -r NewPipe.apk: exit status 1: java.io.IOException: Requested internal only, but not enough space at android.util.ExceptionUtils.wrap(ExceptionUtils.java:34)")
	if code(spaceErr) != "mobile_install_space" {
		t.Fatalf("expected mobile_install_space, got %q (%v)", code(spaceErr), spaceErr)
	}
	abiErr := formatADBInstallError("legacy.apk", "Failure [INSTALL_FAILED_NO_MATCHING_ABIS: Failed to extract native libraries]")
	if code(abiErr) != "mobile_apk_abi" {
		t.Fatalf("expected mobile_apk_abi, got %q (%v)", code(abiErr), abiErr)
	}
	downgradeErr := formatADBInstallError("app.apk", "Failure [INSTALL_FAILED_VERSION_DOWNGRADE]")
	if code(downgradeErr) != "mobile_install" {
		t.Fatalf("expected mobile_install, got %q (%v)", code(downgradeErr), downgradeErr)
	}
	rawErr := formatADBInstallError("app.apk", "adb: failed to install: INSTALL_FAILED_INVALID_APK\n\tat com.android.server.pm.Install.run(Install.java:10)")
	if strings.Contains(rawErr.Error(), "com.android.server.pm") {
		t.Fatalf("expected stacktrace to be stripped, got %q", rawErr.Error())
	}
}

func TestPointIn(t *testing.T) {
	r := Rect{100, 200, 300, 260}
	for i := 0; i < 200; i++ {
		x, y := pointIn(r)
		if x < r.X1 || x >= r.X2 || y < r.Y1 || y >= r.Y2 {
			t.Fatalf("(%d,%d) outside %+v", x, y, r)
		}
	}
}

func code(err error) string {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

// Refusals happen before adb runs: the client here has no adb at all.
func TestRefusals(t *testing.T) {
	d := &Device{ADB: adb.Client{Bin: "/nonexistent/adb", Port: 1}}
	ctx := context.Background()
	if err := d.Tap(ctx, "m1", 0, 0); code(err) != "mobile_ref" {
		t.Errorf("unknown ref: %v", err)
	}
	if err := d.Tap(ctx, "", -1, 5); code(err) != "mobile_point" {
		t.Errorf("negative point: %v", err)
	}
	if err := d.PressKey(ctx, "power"); code(err) != "mobile_key" {
		t.Errorf("power key: %v", err)
	}
	if err := d.Launch(ctx, "com.x; reboot"); code(err) != "mobile_package" {
		t.Errorf("package: %v", err)
	}
	if _, err := d.Push(ctx, "/tmp/a'$(id).mp4"); code(err) != "mobile_file_name" {
		t.Errorf("file name: %v", err)
	}
	for _, u := range []string{"javascript:alert(1)", "https://x.example/'; reboot; '", "file:///sdcard", "https://x.example/\nreboot"} {
		if err := d.OpenURL(ctx, u); code(err) != "mobile_url" {
			t.Errorf("%q: %v", u, err)
		}
	}
	if err := d.Swipe(ctx, "sideways", 0, 0, 0, 0); err == nil {
		t.Error("sideways swipe")
	}
	if err := d.Install(ctx, "/tmp/app.zip"); code(err) != "mobile_apk" {
		t.Errorf("install zip: %v", err)
	}
}

func TestSplitAPKBundles(t *testing.T) {
	dir := t.TempDir()
	writeZip := func(name string, files map[string]string) string {
		p := filepath.Join(dir, name)
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		zw := zip.NewWriter(f)
		for k, v := range files {
			w, err := zw.Create(k)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write([]byte(v)); err != nil {
				t.Fatal(err)
			}
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		return p
	}

	apks := writeZip("app.xapk", map[string]string{
		"base.apk":                "base-bytes",
		"splits/config.arm64.apk": "arm64-bytes",
		"manifest.json":           "{}",
		"icon.png":                "png",
		"__MACOSX/._base.apk":     "junk",
		".hidden.apk":             "junk",
	})
	outDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		t.Fatal(err)
	}
	splits, err := extractSplitAPKs(apks, outDir)
	if err != nil || len(splits) != 2 {
		t.Fatalf("extractSplitAPKs = %v, %v", splits, err)
	}

	withOBB := writeZip("game.xapk", map[string]string{
		"base.apk":                   "base",
		"Android/obb/com.x/main.obb": "obb",
	})
	if _, err := extractSplitAPKs(withOBB, outDir); code(err) != "mobile_apk" {
		t.Fatalf("expected mobile_apk for obb bundle, got %v", err)
	}

	empty := writeZip("empty.apks", map[string]string{"manifest.json": "{}"})
	if _, err := extractSplitAPKs(empty, outDir); code(err) != "mobile_apk" {
		t.Fatalf("expected mobile_apk for empty bundle, got %v", err)
	}

	bad := filepath.Join(dir, "encrypted.apkm")
	os.WriteFile(bad, []byte("not-a-zip"), 0o600)
	if _, err := extractSplitAPKs(bad, outDir); code(err) != "mobile_apk" {
		t.Fatalf("expected mobile_apk for non-zip bundle, got %v", err)
	}
}

func TestKeyboardBound(t *testing.T) {
	for _, c := range []struct {
		dump string
		want bool
	}{
		{"  mCurId=com.android.adbkeyboard/.AdbIME\n  mBoundToMethod=true\n", true},
		{"  mCurId=com.android.adbkeyboard/.AdbIME\n  mBoundToMethod=false\n", false},
		{"  mCurId=com.android.inputmethod.latin/.LatinIME\n  mBoundToMethod=true\n", false},
		// Android 9 puts them on one line.
		{"  mCurId=com.android.adbkeyboard/.AdbIME mHaveConnection=true mBoundToMethod=true", true},
		{"", false},
	} {
		if got := keyboardBound(c.dump); got != c.want {
			t.Errorf("keyboardBound(%q) = %v", c.dump, got)
		}
	}
}

func TestEnsureAuroraSkipsWhenInstalled(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "adb")
	script := "#!/bin/sh\necho 'package:/data/app/com.aurora.store/base.apk'\n"
	if runtime.GOOS == "windows" {
		bin += ".bat"
		script = "@echo package:/data/app/com.aurora.store/base.apk\r\n"
	}
	// Fake adb that reports Aurora Store is already installed on pm path.
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	d := &Device{ADB: adb.Client{Bin: bin, Port: 5555}, Runtime: dir}
	if err := d.EnsureAurora(context.Background()); err != nil {
		t.Fatalf("EnsureAurora should skip download/install when already present: %v", err)
	}
}

func TestCheckAPKABI(t *testing.T) {
	dir := t.TempDir()
	makeAPK := func(name string, entries ...string) string {
		p := filepath.Join(dir, name)
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		zw := zip.NewWriter(f)
		for _, e := range entries {
			w, err := zw.Create(e)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte("elf"))
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
		return p
	}

	universal := makeAPK("universal.apk", "AndroidManifest.xml", "classes.dex")
	arm64 := makeAPK("arm64.apk", "AndroidManifest.xml", "lib/arm64-v8a/libapp.so")
	arm32 := makeAPK("arm32.apk", "AndroidManifest.xml", "lib/armeabi-v7a/libapp.so")
	x86Only := makeAPK("x86.apk", "AndroidManifest.xml", "lib/x86_64/libapp.so")

	if got := apkNativeABIs([]string{universal}); len(got) != 0 {
		t.Fatalf("universal ABIs = %v, want empty", got)
	}
	if err := checkAPKABI("universal.apk", apkNativeABIs([]string{universal}), []string{"arm64-v8a"}); err != nil {
		t.Fatalf("universal should pass: %v", err)
	}
	if err := checkAPKABI("arm64.apk", apkNativeABIs([]string{arm64}), []string{"arm64-v8a"}); err != nil {
		t.Fatalf("arm64 should pass on arm64 device: %v", err)
	}
	if err := checkAPKABI("split.xapk", apkNativeABIs([]string{universal, arm64}), []string{"arm64-v8a"}); err != nil {
		t.Fatalf("split bundle with arm64 split should pass: %v", err)
	}
	if err := checkAPKABI("arm32.apk", apkNativeABIs([]string{arm32}), []string{"arm64-v8a"}); code(err) != "mobile_apk_abi" {
		t.Fatalf("arm32-only on arm64-only device: got %v, want mobile_apk_abi", err)
	}
	if err := checkAPKABI("x86.apk", apkNativeABIs([]string{x86Only}), []string{"arm64-v8a"}); code(err) != "mobile_apk_abi" {
		t.Fatalf("x86_64-only on arm64 device: got %v, want mobile_apk_abi", err)
	}
	if err := checkAPKABI("x86.apk", apkNativeABIs([]string{x86Only}), []string{"x86_64", "arm64-v8a"}); err != nil {
		t.Fatalf("x86_64 APK on x86_64 emulator should pass: %v", err)
	}
}
