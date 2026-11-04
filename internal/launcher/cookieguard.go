// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/D0bby999/anyprofile/internal/store"
)

// A browser deletes every cookie it cannot decrypt. For an engine that keeps
// its own key (AnyBrowser), the app never sees that key: a launch from a build
// or a tool with a different key -- a test binary, --use-mock-keychain, another
// Chromium pointed at this folder -- reads a whole jar it cannot open and
// throws it away. That is silent, and it is only noticed when every account is
// signed out at once.
//
// Two cheap steps turn it into a recoverable event. Before a launch the jar is
// copied aside (tens of kB); at the next launch, if the jar has lost most of
// its cookies, the newest copy goes back before the browser reads it, so the
// sessions come back instead of a login page.
const (
	cookieBackupKeep   = 10
	cookieBackupMaxAge = 14 * 24 * time.Hour
	// cookieLostShare is the share of the jar that has to disappear between two
	// launches before the loss counts as an emptied jar. Sites sign out their
	// own cookies, and browsing drops a few; seventy-five percent at once is not
	// something a session does by itself.
	cookieLostShare = 0.75
)

// cookieState is what one launch leaves for the next.
type cookieState struct {
	Count  int       `json:"count"`
	Backup string    `json:"backup"`
	At     time.Time `json:"at"`
}

// cookieGuard copies the profile's cookie jar and puts the last copy back when
// the jar was emptied between launches. It never fails a launch: a profile
// without a jar, or with one it cannot read, is left exactly as it is.
func (l *Launcher) cookieGuard(prof store.Profile, udd string) {
	if l.log == nil {
		return
	}
	db := filepath.Join(udd, "Default", "Cookies")
	n, err := countCookies(db)
	if err != nil {
		l.log.Debug("cookie jar unreadable", "profile", prof.ID, "err", err)
		return
	}
	dir := filepath.Join(l.dirs.Logs(), "cookie-backups")
	statePath := filepath.Join(dir, prof.ID+".json")
	var prev cookieState
	if b, err := os.ReadFile(statePath); err == nil {
		_ = json.Unmarshal(b, &prev)
	}
	if prev.Count > 0 && float64(n) < float64(prev.Count)*(1-cookieLostShare) {
		if l.restoreJar(prof, dir, db, prev) {
			n = prev.Count
		}
	}
	if n == 0 {
		return // a new profile, or one that never signed in: nothing worth keeping
	}
	if err := l.backUpJar(prof, dir, db, statePath, n); err != nil {
		l.log.Warn("cookie backup", "profile", prof.ID, "err", err)
	}
}

// restoreJar puts the newest copy back over the emptied jar. The emptied file is
// kept beside the copies: what emptied it is worth being able to look at.
func (l *Launcher) restoreJar(prof store.Profile, dir, db string, prev cookieState) bool {
	if prev.Backup == "" || time.Since(prev.At) > cookieBackupMaxAge {
		return false
	}
	src := filepath.Join(dir, prev.Backup)
	if _, err := os.Stat(src); err != nil {
		return false
	}
	aside := filepath.Join(dir, fmt.Sprintf("%s-emptied-%s.db", prof.ID, time.Now().UTC().Format("20060102-150405")))
	if err := copyFile(db, aside); err != nil {
		l.log.Warn("keep the emptied cookie jar", "profile", prof.ID, "err", err)
	}
	// A leftover journal belongs to the emptied jar and would roll the copy back.
	os.Remove(db + "-journal")
	if err := copyFile(src, db); err != nil {
		l.log.Warn("restore cookie jar", "profile", prof.ID, "err", err)
		return false
	}
	l.log.Warn("cookie jar was emptied between launches; the last copy is back",
		"profile", prof.ID, "cookies", prev.Count, "backup", prev.Backup)
	return true
}

// backUpJar copies the jar aside and records its size for the next launch.
func (l *Launcher) backUpJar(prof store.Profile, dir, db, statePath string, n int) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	name := fmt.Sprintf("%s-%s.db", prof.ID, time.Now().UTC().Format("20060102-150405"))
	tmp := filepath.Join(dir, "."+name+".tmp")
	if err := copyFile(db, tmp); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, filepath.Join(dir, name)); err != nil {
		os.Remove(tmp)
		return err
	}
	pruneJars(dir, prof.ID)
	b, err := json.Marshal(cookieState{Count: n, Backup: name, At: time.Now()})
	if err != nil {
		return err
	}
	return os.WriteFile(statePath, b, 0o600)
}

// pruneJars keeps the newest cookieBackupKeep copies of one profile.
func pruneJars(dir, profileID string) {
	matches, err := filepath.Glob(filepath.Join(dir, profileID+"-*.db"))
	if err != nil || len(matches) <= cookieBackupKeep {
		return
	}
	sort.Sort(sort.Reverse(sort.StringSlice(matches))) // the name carries the time
	for _, old := range matches[cookieBackupKeep:] {
		os.Remove(old)
	}
}

// countCookies reads how many cookies the jar holds. A missing file is an empty
// jar, not an error.
func countCookies(path string) (int, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(2000)")
	if err != nil {
		return 0, err
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM cookies`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// copyFile writes src to dst with a fresh inode, so a reader that already has
// the old file open (the browser, a snapshot) sees no half-written jar.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	return out.Close()
}
