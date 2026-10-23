// SPDX-License-Identifier: AGPL-3.0-or-later

package control

import (
	"context"
	"errors"
	"os"
	"regexp"
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
}
