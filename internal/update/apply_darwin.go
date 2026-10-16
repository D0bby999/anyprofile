// SPDX-License-Identifier: AGPL-3.0-or-later

package update

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/D0bby999/anyprofile/internal/engine"
)

// Asset on macOS: a zip holding AnyProfile.app (ditto -c -k --keepParent).
const (
	bundleName = "AnyProfile.app"
	bundleID   = "dev.anyprofile.app"
	stagingDir = ".AnyProfile-update"
	oldBundle  = ".AnyProfile-old.app"
)

var ErrTranslocated = errors.New("macOS runs AnyProfile from a read-only copy; move AnyProfile.app to Applications, open it from there and update again")

// executable is os.Executable; tests point it into a copied bundle.
var executable = os.Executable

// installedApp is the .app bundle this process runs from.
func installedApp() (string, error) {
	exe, err := executable()
	if err != nil {
		return "", err
	}
	if strings.Contains(exe, "/AppTranslocation/") {
		return "", ErrTranslocated
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return "", err
	}
	app := filepath.Dir(filepath.Dir(filepath.Dir(exe))) // .app/Contents/MacOS/AnyProfile
	if !strings.HasSuffix(app, ".app") {
		return "", fmt.Errorf("AnyProfile is not running from an app bundle (%s)", exe)
	}
	if strings.Contains(app, "/AppTranslocation/") {
		return "", ErrTranslocated
	}
	return app, nil
}

// Apply swaps the installed app for the one in the verified zip pkg, then
// starts a shell that waits for pid (this app) to exit, deletes the old copy
// and opens the new one. The caller quits right after. A bundle that is not
// AnyProfile's, or whose code signature is broken, is refused before the swap.
func Apply(pkg string, pid int) error {
	app, err := installedApp()
	if err != nil {
		return err
	}
	if err := swap(app, pkg); err != nil {
		return err
	}
	old := filepath.Join(filepath.Dir(app), oldBundle)
	relaunch := exec.Command("/bin/sh", "-c",
		`while kill -0 "$0" 2>/dev/null; do sleep 0.2; done; rm -rf "$2"; exec /usr/bin/open "$1"`,
		strconv.Itoa(pid), app, old)
	relaunch.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return relaunch.Start()
}

// swap puts the app in pkg where app is, keeping the old one beside it as
// oldBundle. Nothing changes unless the new bundle checks out.
func swap(app, pkg string) error {
	parent := filepath.Dir(app)
	staging := filepath.Join(parent, stagingDir)
	os.RemoveAll(staging)
	defer os.RemoveAll(staging)
	if err := engine.ExtractZip(pkg, staging); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("cannot write in %s; move AnyProfile.app to a folder you own, such as Applications", parent)
		}
		return fmt.Errorf("unpack update: %w", err)
	}
	next := filepath.Join(staging, bundleName)
	if err := checkBundle(next); err != nil {
		return err
	}
	old := filepath.Join(parent, oldBundle)
	os.RemoveAll(old)
	if err := os.Rename(app, old); err != nil {
		return fmt.Errorf("move the old app aside: %w", err)
	}
	if err := os.Rename(next, app); err != nil {
		os.Rename(old, app)
		return fmt.Errorf("put the new app in place: %w", err)
	}
	return nil
}

// checkBundle requires an intact code signature on AnyProfile's bundle id.
func checkBundle(app string) error {
	if out, err := exec.Command("/usr/bin/codesign", "--verify", "--deep", "--strict", app).CombinedOutput(); err != nil {
		return fmt.Errorf("the update's code signature is broken: %s", strings.TrimSpace(string(out)))
	}
	out, _ := exec.Command("/usr/bin/codesign", "-dv", app).CombinedOutput()
	if !strings.Contains(string(out), "Identifier="+bundleID+"\n") {
		return errors.New("the update is not AnyProfile")
	}
	return nil
}

// CleanStale removes what an interrupted update left beside the app.
func CleanStale() {
	if app, err := installedApp(); err == nil {
		os.RemoveAll(filepath.Join(filepath.Dir(app), stagingDir))
		os.RemoveAll(filepath.Join(filepath.Dir(app), oldBundle))
	}
}
