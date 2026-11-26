// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package avo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The watcher moves a newly installed app from screen 1 to the first free cell
// on screen 0, and leaves older items (user drags) alone.
func TestHomePinWatcherMovesNewAppToFirstScreen(t *testing.T) {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 not installed")
	}
	dir := t.TempDir()
	db := filepath.Join(dir, "launcher.db")
	sql := func(q string) string {
		out, err := exec.Command("sqlite3", db, q).CombinedOutput()
		if err != nil {
			t.Fatalf("sqlite3 %q: %v %s", q, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	sql(`CREATE TABLE favorites (_id INTEGER PRIMARY KEY, container INTEGER, screen INTEGER, cellX INTEGER, cellY INTEGER, spanX INTEGER, spanY INTEGER, itemType INTEGER);
INSERT INTO favorites VALUES (1,-100,0,0,0,1,1,0),(2,-100,0,1,0,1,1,0),(3,-100,1,2,2,1,1,0);`)
	script := filepath.Join(dir, "homepin.sh")
	if err := os.WriteFile(script, []byte(homePinWatcherScript), 0o700); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(dir, "lastid")
	if err := os.WriteFile(state, []byte("3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Play Store installs a new app; Launcher3 drops it on screen 1.
	sql(`INSERT INTO favorites VALUES (4,-100,1,0,0,1,1,0);`)

	cmd := exec.Command("sh", script, "once")
	cmd.Env = append(os.Environ(), "AP_HOMEPIN_DB_DIR="+dir, "AP_HOMEPIN_PREFS="+filepath.Join(dir, "none.xml"),
		"AP_HOMEPIN_STATE="+state, "AP_HOMEPIN_SETTLE=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("watcher: %v %s", err, out)
	}
	if got := sql(`SELECT screen||','||cellX||','||cellY FROM favorites WHERE _id=4;`); got != "0,2,0" {
		t.Fatalf("new app at %s, want 0,2,0", got)
	}
	if got := sql(`SELECT screen FROM favorites WHERE _id=3;`); got != "1" {
		t.Fatalf("old item moved to screen %s", got)
	}
	if b, _ := os.ReadFile(state); strings.TrimSpace(string(b)) != "4" {
		t.Fatalf("state = %q, want 4", b)
	}
}
