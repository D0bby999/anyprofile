// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin && vmintegration

package vm

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/control"
)

// Drives a real VM the way the MCP tools do:
// go test -tags vmintegration -run TestControl -v ./internal/mobile/vm/
// ANYPROFILE_VM_DUMPS, when set, receives the screens' uiautomator XML.
func TestControl(t *testing.T) {
	rtDir := os.Getenv("ANYPROFILE_VM_RUNTIME")
	if rtDir == "" {
		t.Skip("ANYPROFILE_VM_RUNTIME not set")
	}
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	drv := New(rtDir, t.TempDir(), log)
	d := mobile.Device{ID: "ctl" + time.Now().Format("150405"), Name: "Control", RAMMB: 3072, CPUs: 4}
	ref, _, err := drv.Create(ctx, d)
	if err != nil {
		t.Fatal(err)
	}
	d.Ref = ref
	defer drv.Delete(ctx, d)
	run, err := drv.Start(ctx, d, mobile.StartOptions{Timezone: "Asia/Ho_Chi_Minh", Locale: "vi-VN"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { drv.Stop(ctx, d); <-run.Done }()
	dev := &control.Device{ADB: drv.proc(d.ID).adb, Runtime: t.TempDir()}
	dump := func(name string) {
		if dir := os.Getenv("ANYPROFILE_VM_DUMPS"); dir != "" {
			out, _ := dev.ADB.Shell(ctx, "uiautomator dump /data/local/tmp/x.xml >/dev/null && cat /data/local/tmp/x.xml")
			os.WriteFile(filepath.Join(dir, name+".xml"), []byte(out), 0o600)
		}
	}

	time.Sleep(5 * time.Second) // the launcher settles after boot
	home, err := dev.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	dump("launcher")
	t.Logf("launcher:\n%s", head(home, 15))
	if !strings.Contains(home, "[ref=m1]") {
		t.Error("nothing to act on on the home screen")
	}

	if err := dev.Launch(ctx, browserPkg); err != nil {
		t.Fatal(err)
	}
	if ok, _ := dev.WaitFor(ctx, "editable", false, 20); !ok {
		s, _ := dev.Snapshot(ctx)
		t.Fatalf("no text field in the browser:\n%s", head(s, 30))
	}
	page, _ := dev.Snapshot(ctx)
	dump("browser")
	m := regexp.MustCompile(`\[ref=(m\d+)\][^\n]*editable`).FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("no editable ref:\n%s", head(page, 30))
	}
	const words = "Xin chào 😀 tiếng Việt"
	if err := dev.Type(ctx, m[1], words, false); err != nil {
		t.Fatal(err)
	}
	typed, _ := dev.Snapshot(ctx)
	dump("typed")
	if !strings.Contains(typed, "Xin chào") || !strings.Contains(typed, "tiếng Việt") {
		t.Errorf("typed text not on screen:\n%s", head(typed, 30))
	}
	if ime, _ := dev.ADB.Shell(ctx, "settings get secure default_input_method"); strings.Contains(ime, "adbkeyboard") {
		t.Error("ADBKeyBoard left as the input method")
	}

	shot, err := dev.Screenshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cfg, err := jpeg.DecodeConfig(bytes.NewReader(shot)); err != nil || cfg.Width != screenW {
		t.Fatalf("screenshot %+v %v", cfg, err)
	}

	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(1, 1, color.RGBA{255, 0, 0, 255})
	local := filepath.Join(t.TempDir(), "anyprofile-test.png")
	f, _ := os.Create(local)
	png.Encode(f, img)
	f.Close()
	remote, err := dev.Push(ctx, local)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("pushed to %s", remote)
	found := false
	for i := 0; i < 10 && !found; i++ {
		out, _ := dev.ADB.Shell(ctx, "content query --uri content://media/external/images/media --projection _display_name")
		found = strings.Contains(out, "anyprofile-test.png")
		time.Sleep(time.Second)
	}
	if !found {
		t.Error("the pushed picture is not in the media store")
	}

	if err := dev.OpenURL(ctx, "https://example.com/"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := dev.WaitFor(ctx, "documentation examples", false, 30); !ok {
		s, _ := dev.Snapshot(ctx)
		t.Errorf("example.com did not open:\n%s", head(s, 20))
	}

	if err := dev.Swipe(ctx, "up", 0, 0, 0, 0); err != nil {
		t.Error(err)
	}
	if err := dev.PressKey(ctx, "home"); err != nil {
		t.Error(err)
	}
	if err := dev.Tap(ctx, "m999", 0, 0); err == nil {
		t.Error("a ref that is not there was tapped")
	}
}

func head(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
