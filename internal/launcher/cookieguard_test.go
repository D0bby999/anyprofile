// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/store"
)

// cookieJar writes a Cookies database holding rows cookies, as the browser
// leaves one behind.
func cookieJar(t *testing.T, path string, rows int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS cookies (host_key TEXT NOT NULL, name TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM cookies`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < rows; i++ {
		if _, err := db.Exec(`INSERT INTO cookies (host_key, name) VALUES (?, ?)`, ".example.com", "c"+string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
	}
}

func jarRows(t *testing.T, path string) int {
	t.Helper()
	n, err := countCookies(path)
	if err != nil {
		t.Fatalf("count %s: %v", path, err)
	}
	return n
}

func guardFor(t *testing.T) (*Launcher, paths.Dirs) {
	t.Helper()
	dirs := paths.Dirs{Root: t.TempDir()}
	return &Launcher{dirs: dirs, log: slog.New(slog.NewTextHandler(io.Discard, nil))}, dirs
}

func TestCookieGuardCopiesTheJar(t *testing.T) {
	l, dirs := guardFor(t)
	prof := store.Profile{ID: "p1"}
	udd := dirs.Profile(prof.ID)
	db := filepath.Join(udd, "Default", "Cookies")
	cookieJar(t, db, 5)

	l.cookieGuard(prof, udd)

	state := filepath.Join(dirs.Logs(), "cookie-backups", prof.ID+".json")
	b, err := os.ReadFile(state)
	if err != nil {
		t.Fatalf("no state written: %v", err)
	}
	var st cookieState
	if err := json.Unmarshal(b, &st); err != nil {
		t.Fatal(err)
	}
	if st.Count != 5 || st.Backup == "" {
		t.Fatalf("state = %+v, want 5 cookies and a copy name", st)
	}
	if _, err := os.Stat(filepath.Join(dirs.Logs(), "cookie-backups", st.Backup)); err != nil {
		t.Fatalf("the copy is missing: %v", err)
	}
}

func TestCookieGuardReplacesAnEmptiedJar(t *testing.T) {
	l, dirs := guardFor(t)
	prof := store.Profile{ID: "p1"}
	udd := dirs.Profile(prof.ID)
	db := filepath.Join(udd, "Default", "Cookies")
	cookieJar(t, db, 5)
	l.cookieGuard(prof, udd) // the good jar is copied aside

	cookieJar(t, db, 0) // something opened the folder with another key and the browser dropped the lot
	l.cookieGuard(prof, udd)

	if got := jarRows(t, db); got != 5 {
		t.Fatalf("jar holds %d cookies after the guard, want the 5 that were restored", got)
	}
	emptied, _ := filepath.Glob(filepath.Join(dirs.Logs(), "cookie-backups", prof.ID+"-emptied-*.db"))
	if len(emptied) != 1 {
		t.Fatalf("the emptied jar was not kept beside the copies: %v", emptied)
	}
}

func TestCookieGuardLeavesASmallerJarAlone(t *testing.T) {
	l, dirs := guardFor(t)
	prof := store.Profile{ID: "p1"}
	udd := dirs.Profile(prof.ID)
	db := filepath.Join(udd, "Default", "Cookies")
	cookieJar(t, db, 8)
	l.cookieGuard(prof, udd)

	cookieJar(t, db, 4) // the sites signed themselves out: two thirds are left
	l.cookieGuard(prof, udd)

	if got := jarRows(t, db); got != 4 {
		t.Fatalf("jar holds %d cookies, want the 4 that browsing left", got)
	}
}

func TestCookieGuardWithoutAJarDoesNothing(t *testing.T) {
	l, dirs := guardFor(t)
	prof := store.Profile{ID: "p1"}
	udd := dirs.Profile(prof.ID)
	if err := os.MkdirAll(filepath.Join(udd, "Default"), 0o700); err != nil {
		t.Fatal(err)
	}

	l.cookieGuard(prof, udd)

	if _, err := os.Stat(filepath.Join(dirs.Logs(), "cookie-backups", prof.ID+".json")); !os.IsNotExist(err) {
		t.Fatalf("a profile that never signed in got a copy: %v", err)
	}
}

func TestCookieGuardPrunesOldCopies(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"p1-20260101-000000.db", "p1-20260102-000000.db", "p1-20260103-000000.db"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < cookieBackupKeep+2; i++ {
		name := "p1-2026020" + string(rune('0'+i)) + "-000000.db"
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	pruneJars(dir, "p1")

	left, _ := filepath.Glob(filepath.Join(dir, "p1-*.db"))
	if len(left) != cookieBackupKeep {
		t.Fatalf("%d copies left, want %d", len(left), cookieBackupKeep)
	}
	for _, name := range left {
		if strings.HasSuffix(name, "p1-20260101-000000.db") {
			t.Fatalf("the oldest copy survived: %v", left)
		}
	}
}
