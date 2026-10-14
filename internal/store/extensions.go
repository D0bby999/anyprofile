// SPDX-License-Identifier: AGPL-3.0-or-later

package store

// Extension is one unpacked extension in the library.
type Extension struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	CreatedAt string `json:"createdAt"`
	// Profiles counts the profiles that load it.
	Profiles int `json:"profiles"`
}

func (s *Store) CreateExtension(e *Extension) error {
	e.CreatedAt = now()
	_, err := s.db.Exec(`INSERT INTO extensions (id, name, version, created_at) VALUES (?,?,?,?)`,
		e.ID, e.Name, e.Version, e.CreatedAt)
	return err
}

func (s *Store) ListExtensions() ([]Extension, error) {
	rows, err := s.db.Query(`SELECT e.id, e.name, e.version, e.created_at,
		(SELECT COUNT(*) FROM profile_extensions pe JOIN profiles p ON p.id = pe.profile_id
		 WHERE pe.extension_id = e.id AND p.deleted_at IS NULL)
		FROM extensions e ORDER BY e.name COLLATE NOCASE, e.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Extension{}
	for rows.Next() {
		var e Extension
		if err := rows.Scan(&e.ID, &e.Name, &e.Version, &e.CreatedAt, &e.Profiles); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// DeleteExtension removes the extension and every profile's use of it.
func (s *Store) DeleteExtension(id string) error {
	res, err := s.db.Exec(`DELETE FROM extensions WHERE id = ?`, id)
	return affected(res, err)
}

// SetProfileExtension turns one extension on or off for one profile.
func (s *Store) SetProfileExtension(profileID, extID string, on bool) error {
	var err error
	if on {
		_, err = s.db.Exec(`INSERT OR IGNORE INTO profile_extensions (profile_id, extension_id) VALUES (?, ?)`, profileID, extID)
	} else {
		_, err = s.db.Exec(`DELETE FROM profile_extensions WHERE profile_id = ? AND extension_id = ?`, profileID, extID)
	}
	return err
}

// extensionsByProfile maps profile ids to the extension ids they load.
func (s *Store) extensionsByProfile(id string) (map[string][]string, error) {
	q := `SELECT profile_id, extension_id FROM profile_extensions`
	var args []any
	if id != "" {
		q += ` WHERE profile_id = ?`
		args = append(args, id)
	}
	rows, err := s.db.Query(q+` ORDER BY rowid`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var pid, eid string
		if err := rows.Scan(&pid, &eid); err != nil {
			return nil, err
		}
		out[pid] = append(out[pid], eid)
	}
	return out, rows.Err()
}
