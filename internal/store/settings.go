// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import "database/sql"

// SettingOnOpen says what a profile opens with: "restore" (the tabs of last
// time, the default) or "start" (its start pages, every time).
const SettingOnOpen = "on_open"

// GetSetting returns the stored value for key, or "" and nil when unset.
func (s *Store) GetSetting(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

// SetSetting stores a key/value pair, replacing any previous value.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}
