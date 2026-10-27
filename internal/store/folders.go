// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"database/sql"
	"errors"
	"strings"
)

// Folder groups profiles by channel, client or purpose. A profile is in
// at most one folder; FolderID "" on a profile means unfiled.
type Folder struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
}

// ErrFolderNameTaken reports a create or rename that collides with an
// existing folder name (compared trimmed and case-insensitively).
var ErrFolderNameTaken = errors.New("folder name already exists")

// ListFolders returns every folder in display order.
func (s *Store) ListFolders() ([]Folder, error) {
	rows, err := s.db.Query(`SELECT id, name, color, sort_order, created_at FROM folders ORDER BY sort_order, created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Folder{}
	for rows.Next() {
		var f Folder
		if err := rows.Scan(&f.ID, &f.Name, &f.Color, &f.SortOrder, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// GetFolder returns one folder, or ErrNotFound.
func (s *Store) GetFolder(id string) (Folder, error) {
	var f Folder
	err := s.db.QueryRow(`SELECT id, name, color, sort_order, created_at FROM folders WHERE id = ?`, id).
		Scan(&f.ID, &f.Name, &f.Color, &f.SortOrder, &f.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return f, ErrNotFound
	}
	return f, err
}

// folderNameTaken reports whether another folder already uses the name,
// compared trimmed and case-insensitively in Go: SQLite's lower() folds
// ASCII only, so "Khách A" and "KHÁCH A" would slip past a SQL check.
// Folders are few, so reading their names is cheap.
func folderNameTaken(tx *sql.Tx, name, exceptID string) (bool, error) {
	rows, err := tx.Query(`SELECT id, name FROM folders`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, other string
		if err := rows.Scan(&id, &other); err != nil {
			return false, err
		}
		if id != exceptID && strings.EqualFold(strings.TrimSpace(other), strings.TrimSpace(name)) {
			return true, nil
		}
	}
	return false, rows.Err()
}

// CreateFolder inserts a folder; the name must be non-empty and unique.
func (s *Store) CreateFolder(f *Folder) error {
	f.Name = strings.TrimSpace(f.Name)
	if f.ID == "" || f.Name == "" {
		return errors.New("folder needs id and name")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if taken, err := folderNameTaken(tx, f.Name, ""); err != nil {
		return err
	} else if taken {
		return ErrFolderNameTaken
	}
	f.CreatedAt = now()
	_, err = tx.Exec(`INSERT INTO folders (id, name, color, sort_order, created_at) VALUES (?, ?, ?, ?, ?)`,
		f.ID, f.Name, f.Color, f.SortOrder, f.CreatedAt)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// RenameFolder changes a folder's name, keeping the uniqueness rule.
func (s *Store) RenameFolder(id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("folder name must not be empty")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if taken, err := folderNameTaken(tx, name, id); err != nil {
		return err
	} else if taken {
		return ErrFolderNameTaken
	}
	res, err := tx.Exec(`UPDATE folders SET name = ? WHERE id = ?`, name, id)
	if err := affected(res, err); err != nil {
		return err
	}
	return tx.Commit()
}

// SetFolderColor changes a folder's color ("" for none).
func (s *Store) SetFolderColor(id, color string) error {
	res, err := s.db.Exec(`UPDATE folders SET color = ? WHERE id = ?`, color, id)
	return affected(res, err)
}

// ReorderFolders sets sort_order from the given id order; ids not in the
// list keep their place after the listed ones.
func (s *Store) ReorderFolders(ids []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT id FROM folders ORDER BY sort_order, created_at, id`)
	if err != nil {
		return err
	}
	var existing []string
	known := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		existing = append(existing, id)
		known[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	order := make([]string, 0, len(existing))
	for _, id := range ids {
		if !known[id] {
			return errors.New("folder order contains an unknown or repeated id")
		}
		order = append(order, id)
		known[id] = false
	}
	for _, id := range existing {
		if known[id] {
			order = append(order, id)
		}
	}
	for i, id := range order {
		if _, err := tx.Exec(`UPDATE folders SET sort_order = ? WHERE id = ?`, i, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeleteFolder removes a folder after moving its profiles to moveTo
// ("" unfiles them), in one transaction: a folder delete can never lose
// or orphan a profile. Trashed profiles move too, so a restore lands in
// the target folder.
func (s *Store) DeleteFolder(id, moveTo string) error {
	if moveTo == id {
		return errors.New("cannot move profiles into the folder being deleted")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if moveTo != "" {
		var n int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM folders WHERE id = ?`, moveTo).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
	}
	if _, err := tx.Exec(`UPDATE profiles SET folder_id = ? WHERE folder_id = ?`, nullStr(moveTo), id); err != nil {
		return err
	}
	res, err := tx.Exec(`DELETE FROM folders WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// SetProfileFolder files a profile, or unfiles it when folderID is "".
func (s *Store) SetProfileFolder(profileID, folderID string) error {
	res, err := s.db.Exec(`UPDATE profiles SET folder_id = ?, updated_at = ? WHERE id = ?`,
		nullStr(folderID), now(), profileID)
	return affected(res, err)
}

// SetProfilesFolder moves a deduplicated batch atomically. A missing profile or
// destination rolls back every change; callers never see a half-finished move.
func (s *Store) SetProfilesFolder(ids []string, folderID string) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if folderID != "" {
		var id string
		if err := tx.QueryRow(`SELECT id FROM folders WHERE id = ?`, folderID).Scan(&id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, ErrNotFound
			}
			return 0, err
		}
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		res, err := tx.Exec(`UPDATE profiles SET folder_id = ?, updated_at = ? WHERE id = ?`, nullStr(folderID), now(), id)
		if err := affected(res, err); err != nil {
			return 0, err
		}
		seen[id] = true
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(seen), nil
}

// FolderCounts counts live profiles per folder id, plus the unfiled
// count under the "" key.
func (s *Store) FolderCounts() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT COALESCE(folder_id, ''), COUNT(*) FROM profiles WHERE deleted_at IS NULL GROUP BY folder_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}
