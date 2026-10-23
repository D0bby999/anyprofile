// SPDX-License-Identifier: AGPL-3.0-or-later

package paths

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultHonoursAbsoluteOverride(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ap-test-root")
	t.Setenv(EnvDataDir, root+string(filepath.Separator))
	d, err := Default()
	if err != nil || d.Root != root {
		t.Fatalf("override = %q %v", d.Root, err)
	}
	t.Setenv(EnvDataDir, "relative/dir")
	d, err = Default()
	if err != nil || !strings.HasSuffix(d.Root, string(filepath.Separator)+"AnyProfile") {
		t.Fatalf("relative override must be ignored, got %q %v", d.Root, err)
	}
}

func TestProfileSnapshotsIsUnderSnapshots(t *testing.T) {
	d := Dirs{Root: filepath.FromSlash("/r")}
	if d.ProfileSnapshots("abc") != filepath.FromSlash("/r/snapshots/abc") || !strings.HasPrefix(d.ProfileSnapshots("abc"), d.Snapshots()+string(filepath.Separator)) {
		t.Fatalf("snapshots = %s", d.ProfileSnapshots("abc"))
	}
}

func TestMobilePaths(t *testing.T) {
	d := Dirs{Root: filepath.FromSlash("/r")}
	for got, want := range map[string]string{
		d.MobileDevice("abc"): "/r/mobile/devices/abc",
		d.MobileBackup("abc"): "/r/mobile/backups/abc",
		d.MobileRuntime():     "/r/mobile/runtime",
	} {
		if got != filepath.FromSlash(want) {
			t.Errorf("%s, want %s", got, want)
		}
	}
}
