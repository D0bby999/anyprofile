// SPDX-License-Identifier: AGPL-3.0-or-later

package fury

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"golang.org/x/sys/unix"
)

// The Widevine CDM may not be redistributed, so AnyProfile ships none. Like
// the Fury agent (agent/src/widevine.rs), it copies the licensed copy from the
// user's own Google Chrome into <user-data-dir>/WidevineCdm/<version>/, where
// the component updater registers it. Writing into the signed core bundle
// instead would break its seal and Gatekeeper would call it damaged.

var errNoChrome = errors.New("no Google Chrome with a Widevine CDM found")

var cdmVersionRe = regexp.MustCompile(`^[0-9.]+$`)

func defaultChromeApps() []string {
	out := []string{"/Applications/Google Chrome.app"}
	if h, err := os.UserHomeDir(); err == nil {
		out = append(out, filepath.Join(h, "Applications/Google Chrome.app"))
	}
	return out
}

// findCDM returns the CDM root (holding manifest.json) inside a Chrome bundle.
func findCDM(chromeApp string) (string, error) {
	versions, err := filepath.Glob(filepath.Join(chromeApp,
		"Contents/Frameworks/Google Chrome Framework.framework/Versions/*/Libraries/WidevineCdm"))
	if err != nil {
		return "", err
	}
	for i := len(versions) - 1; i >= 0; i-- { // newest last in lexical order
		root := versions[i]
		lib := filepath.Join(root, "_platform_specific/mac_arm64/libwidevinecdm.dylib")
		if fileExists(lib) && fileExists(filepath.Join(root, "manifest.json")) {
			return root, nil
		}
	}
	return "", errNoChrome
}

func stageWidevine(chromeApps []string, userDataDir string) error {
	var root string
	for _, app := range chromeApps {
		if r, err := findCDM(app); err == nil {
			root = r
			break
		}
	}
	if root == "" {
		return errNoChrome
	}
	b, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		return err
	}
	var m struct{ Version string }
	if err := json.Unmarshal(b, &m); err != nil || !cdmVersionRe.MatchString(m.Version) {
		return fmt.Errorf("widevine manifest has no usable version")
	}
	dest := filepath.Join(userDataDir, "WidevineCdm", m.Version)
	plat := "_platform_specific/mac_arm64"
	files := []string{
		filepath.Join(plat, "libwidevinecdm.dylib"),
		filepath.Join(plat, "libwidevinecdm.dylib.sig"),
		"manifest.json",
		"LICENSE",
	}
	for _, rel := range files {
		src := filepath.Join(root, rel)
		if !fileExists(src) {
			continue // the .sig and LICENSE are optional
		}
		if err := copyIfChanged(src, filepath.Join(dest, rel)); err != nil {
			return fmt.Errorf("stage widevine %s: %w", rel, err)
		}
	}
	return nil
}

func copyIfChanged(src, dst string) error {
	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if di, err := os.Stat(dst); err == nil && di.Size() == si.Size() {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	os.Remove(dst)
	if unix.Clonefile(src, dst, 0) == nil {
		return nil // APFS clone: no extra space per profile
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}
