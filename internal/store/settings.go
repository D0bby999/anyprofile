// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import "database/sql"

// SettingOnOpen says what a profile opens with: "start" (its start pages,
// every time; the default, also when unset) or "restore" (the tabs of last
// time).
const SettingOnOpen = "on_open"

// SettingProfileOrder is the order the user dragged the profiles into: a
// JSON array of profile IDs.
const SettingProfileOrder = "profile_order"

// SettingMaxConcurrentLaunches is the launch ceiling: how many browsers
// may be starting or running at once, as a decimal integer. Unset or "0"
// means unlimited.
const SettingMaxConcurrentLaunches = "max_concurrent_launches"

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
