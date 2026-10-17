// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// EditPrefs lets edit change a Chromium Preferences file before a launch and
// writes it back when edit reports a change. A file that is not JSON is left
// as it is and reported: overwriting it would lose the profile's settings.
func EditPrefs(path string, edit func(m map[string]any) bool) error {
	m := map[string]any{}
	b, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
	case err != nil:
		return err
	default:
		if err := json.Unmarshal(b, &m); err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
	}
	if !edit(m) {
		return nil
	}
	out, err := json.Marshal(m)
	if err != nil {
		return err
	}
	tmp := path + ".anyprofile-tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// PrefsDict returns m[key] as a dictionary, adding an empty one if missing.
func PrefsDict(m map[string]any, key string) map[string]any {
	d, _ := m[key].(map[string]any)
	if d == nil {
		d = map[string]any{}
		m[key] = d
	}
	return d
}

// StayOutOfChromeSignIn turns off "Allow Chrome sign-in". With it on, the
// browser joins every Google sign-in made on the web (Dice): it asks Google
// for a token of its own, which fails without a real OAuth client, and on the
// next launch its account reconcilor signs the web out to match. The profile
// then lost its Google sign-in at every restart. Off, the browser leaves the
// Google cookies alone, as for anyone who turned that setting off.
func StayOutOfChromeSignIn(m map[string]any) bool {
	signin := PrefsDict(m, "signin")
	changed := false
	// allowed is reset from allowed_on_next_startup at each launch.
	for _, k := range []string{"allowed", "allowed_on_next_startup"} {
		if v, ok := signin[k].(bool); !ok || v {
			signin[k] = false
			changed = true
		}
	}
	return changed
}
