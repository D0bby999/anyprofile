// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// keepDaily is how many daily database backups stay.
const keepDaily = 7

// BackupDaily copies the database to dir/daily-YYYYMMDD.db once per day and
// keeps the newest seven. Migration backups in the same folder are left alone.
func (s *Store) BackupDaily(dir string, now time.Time) error {
	path := filepath.Join(dir, "daily-"+now.Format("20060102")+".db")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if _, err := s.db.Exec(`VACUUM INTO ?`, path); err != nil {
		return err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	all, err := filepath.Glob(filepath.Join(dir, "daily-*.db"))
	if err != nil {
		return err
	}
	sort.Strings(all) // the date in the name sorts them oldest first
	for len(all) > keepDaily {
		if strings.HasPrefix(filepath.Base(all[0]), "daily-") {
			if err := os.Remove(all[0]); err != nil {
				return err
			}
		}
		all = all[1:]
	}
	return nil
}
