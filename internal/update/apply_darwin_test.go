// SPDX-License-Identifier: AGPL-3.0-or-later

package update

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Swaps a copy of the real build (make build) for a zipped copy of itself,
// and refuses a bundle whose signature was broken.
func TestSwapRealBundle(t *testing.T) {
	built, _ := filepath.Abs("../../build/bin/AnyProfile.app")
	if _, err := os.Stat(built); err != nil {
		t.Skip("needs make build")
	}
	root, _ := filepath.EvalSymlinks(t.TempDir())
	app := filepath.Join(root, "Applications", bundleName)
	os.MkdirAll(filepath.Dir(app), 0o700)
	if out, err := exec.Command("/usr/bin/ditto", built, app).CombinedOutput(); err != nil {
		t.Fatalf("copy: %s", out)
	}
	marker := filepath.Join(app, "Contents", "old-marker")
	os.WriteFile(marker, nil, 0o600) // breaks the old copy's seal, which is fine: only the new one is checked
	pkg := filepath.Join(root, "AnyProfile-macos-arm64.zip")
	if out, err := exec.Command("/usr/bin/ditto", "-c", "-k", "--keepParent", built, pkg).CombinedOutput(); err != nil {
		t.Fatalf("zip: %s", out)
	}
	old := executable
	executable = func() (string, error) { return filepath.Join(app, "Contents", "MacOS", "AnyProfile"), nil }
	t.Cleanup(func() { executable = old })

	got, err := installedApp()
	if err != nil || got != app {
		t.Fatalf("installedApp = %q, %v", got, err)
	}
	if err := swap(app, pkg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the old app is still in place")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(app), oldBundle, "Contents", "old-marker")); err != nil {
		t.Fatal("the old app was not kept aside")
	}
	if err := checkBundle(app); err != nil {
		t.Fatalf("new app: %v", err)
	}
	CleanStale()
	if _, err := os.Stat(filepath.Join(filepath.Dir(app), oldBundle)); err == nil {
		t.Fatal("CleanStale left the old app")
	}

	// A tampered bundle is refused and the installed app stays.
	bad := filepath.Join(root, "bad")
	exec.Command("/usr/bin/ditto", built, filepath.Join(bad, bundleName)).Run()
	os.WriteFile(filepath.Join(bad, bundleName, "Contents", "MacOS", "AnyProfile"), []byte("evil"), 0o700)
	badPkg := filepath.Join(root, "bad.zip")
	exec.Command("/usr/bin/ditto", "-c", "-k", "--keepParent", filepath.Join(bad, bundleName), badPkg).Run()
	if err := swap(app, badPkg); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("tampered bundle: %v", err)
	}
	if err := checkBundle(app); err != nil {
		t.Fatalf("installed app damaged by a refused update: %v", err)
	}
}

func TestTranslocatedAppRefused(t *testing.T) {
	old := executable
	executable = func() (string, error) {
		return "/private/var/folders/x/AppTranslocation/ABC/d/AnyProfile.app/Contents/MacOS/AnyProfile", nil
	}
	t.Cleanup(func() { executable = old })
	if _, err := installedApp(); err != ErrTranslocated {
		t.Fatalf("err = %v", err)
	}
}
