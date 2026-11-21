// SPDX-License-Identifier: AGPL-3.0-or-later
//go:build darwin && avointegration

package avo

import (
	"context"
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile"
)

//go:embed testdata/settings-scroll.sh
var settingsScroll string

// Write only disposable device disks; cold preparation runs against a cloned installation.
func TestQuickstartInteractive(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	d := New(filepath.Join(home, "Library/Application Support/AnyProfile/mobile/runtime"), t.TempDir(), nil)
	rt, err := Locate(d.avoRoot)
	if err != nil {
		t.Skip(err)
	}
	if os.Getenv("AVO_BENCH_COLD") == "1" {
		// Clone the installation before removing the disposable copy's prepared base.
		isolated := filepath.Join(t.TempDir(), "avo")
		if out, err := exec.Command("cp", "-cR", d.avoRoot, isolated).CombinedOutput(); err != nil {
			t.Fatalf("clone runtime: %s %v", out, err)
		}
		d.avoRoot = isolated
		rt, err = Locate(isolated)
		if err != nil {
			t.Fatal(err)
		}
		image, err := filepath.EvalSymlinks(rt.ImageDir)
		root, rootErr := filepath.EvalSymlinks(isolated)
		if err != nil || rootErr != nil || !strings.HasPrefix(image, root+string(os.PathSeparator)) {
			t.Fatal("cloned image is not inside the temporary runtime")
		}
		if err := os.RemoveAll(gmsBaseDir(rt)); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
		defer cancel()
		if err := d.Install(ctx, func(stage string, _, _ int64) { t.Logf("install stage=%s", stage) }); err != nil {
			t.Fatal(err)
		}
		t.Logf("base_preparation_ms=%d", time.Since(start).Milliseconds())
	}
	if _, ok := gmsBaseReady(rt); !ok {
		t.Skip("prepared Google Play base required")
	}
	dev := mobile.Device{ID: "bench", Name: "AnyMobile benchmark", Driver: "avo", CPUs: 3, RAMMB: 4096, ScreenW: 780, ScreenH: 1688, Density: 320, WithGMS: true, Preset: "balanced"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	count := 2
	multi := os.Getenv("AVO_BENCH_INSTANCES")
	if multi != "" {
		var err error
		count, err = strconv.Atoi(multi)
		if err != nil || count < 2 || count > 4 {
			t.Fatal("AVO_BENCH_INSTANCES must be 2–4")
		}
	}
	for i := 0; i < count; i++ {
		dev := dev
		if multi != "" {
			dev.ID += strconv.Itoa(i)
		}
		start := time.Now()
		if i == 0 || multi != "" {
			if _, _, err := d.Create(ctx, dev); err != nil {
				t.Fatal(err)
			}
		}
		func() {
			run, err := d.Start(ctx, dev, mobile.StartOptions{})
			if err != nil {
				t.Fatal(err)
			}
			stopDevice := func() {
				stop, c := context.WithTimeout(context.Background(), 30*time.Second)
				defer c()
				if err := d.Stop(stop, dev); err != nil {
					t.Error(err)
				}
			}
			if multi != "" {
				t.Cleanup(stopDevice)
			} else {
				defer stopDevice()
			}
			a, ok := d.ADB(dev)
			if !ok {
				t.Fatal("ADB missing")
			}
			out, err := a.Shell(ctx, "am start -W -a android.settings.SETTINGS")
			if err != nil || !strings.Contains(out, "Status: ok") {
				t.Fatalf("Settings did not open: %s %v", out, err)
			}
			if _, err := a.Shell(ctx, "input keyevent KEYCODE_HOME"); err != nil {
				t.Fatal(err)
			}
			for {
				out, err := a.Shell(ctx, "dumpsys activity activities | grep -im1 resumedactivity")
				if err == nil && strings.Contains(strings.ToLower(out), "launcher") {
					break
				}
				if time.Since(start) > 2*time.Minute {
					t.Fatalf("HOME not confirmed: %s %v", out, err)
				}
				time.Sleep(250 * time.Millisecond)
			}
			t.Logf("cycle=%d interactive_ms=%d", i+1, time.Since(start).Milliseconds())
			if multi != "" {
				rss, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(run.Launch.PID)).Output()
				if err != nil {
					t.Fatal(err)
				}
				out, err := a.Shell(ctx, settingsScroll)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("instances=%d last_vmm_rss_kib=%s settings_scroll=%s", i+1, strings.TrimSpace(string(rss)), out)
				if i+1 == count && os.Getenv("MOBILE_BENCH_SUSTAINED") == "1" {
					frames, err := a.Shell(ctx, "set -- 20\n"+settingsScroll)
					if err != nil {
						t.Fatal(err)
					}
					t.Logf("sustained_scroll=%s", frames)
				}
			} else if i == 0 {
				if _, err := a.Shell(ctx, "echo durable >/data/local/tmp/quickstart-check && sync"); err != nil {
					t.Fatal(err)
				}
			} else if _, err := a.Shell(ctx, "grep -qx durable /data/local/tmp/quickstart-check"); err != nil {
				t.Fatalf("guest write missing after restart: %v", err)
			}
		}()
	}
}
