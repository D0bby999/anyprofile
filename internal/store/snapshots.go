// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"database/sql"
	"errors"
)

// Snapshot is one encrypted copy of a profile folder.
type Snapshot struct {
	ID        int64  `json:"id"`
	ProfileID string `json:"profileId"`
	CreatedAt string `json:"createdAt"`
	Reason    string `json:"reason"` // close, manual, before-restore or before-import
	Bytes     int64  `json:"bytes"`
	File      string `json:"file"` // base name inside snapshots/<profile_id>/
}

const snapshotCols = `id, profile_id, created_at, reason, bytes, file`

func scanSnapshot(row interface{ Scan(...any) error }) (Snapshot, error) {
	var x Snapshot
	err := row.Scan(&x.ID, &x.ProfileID, &x.CreatedAt, &x.Reason, &x.Bytes, &x.File)
	return x, err
}

func (s *Store) AddSnapshot(x *Snapshot) error {
	if x.CreatedAt == "" {
		x.CreatedAt = now()
	}
	res, err := s.db.Exec(`INSERT INTO snapshots (profile_id, created_at, reason, bytes, file) VALUES (?, ?, ?, ?, ?)`,
		x.ProfileID, x.CreatedAt, x.Reason, x.Bytes, x.File)
	if err != nil {
		return err
	}
	x.ID, err = res.LastInsertId()
	return err
}

// Snapshots lists the profile's snapshots, newest first.
func (s *Store) Snapshots(profileID string) ([]Snapshot, error) {
	return s.querySnapshots(`SELECT `+snapshotCols+` FROM snapshots WHERE profile_id = ? ORDER BY id DESC`, profileID)
}

func (s *Store) GetSnapshot(id int64) (Snapshot, error) {
	x, err := scanSnapshot(s.db.QueryRow(`SELECT `+snapshotCols+` FROM snapshots WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return x, ErrNotFound
	}
	return x, err
}

func (s *Store) DeleteSnapshot(id int64) error {
	res, err := s.db.Exec(`DELETE FROM snapshots WHERE id = ?`, id)
	return affected(res, err)
}

// SnapshotsToPrune are the snapshots of one reason beyond the newest keep.
func (s *Store) SnapshotsToPrune(profileID, reason string, keep int) ([]Snapshot, error) {
	return s.querySnapshots(`SELECT `+snapshotCols+` FROM snapshots WHERE profile_id = ? AND reason = ?
		ORDER BY id DESC LIMIT -1 OFFSET ?`, profileID, reason, keep)
}

// AllSnapshots lists every snapshot (for deleting them all).
func (s *Store) AllSnapshots() ([]Snapshot, error) {
	return s.querySnapshots(`SELECT ` + snapshotCols + ` FROM snapshots ORDER BY id`)
}

// SnapshotBytes is the size of every snapshot together.
func (s *Store) SnapshotBytes() (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT COALESCE(SUM(bytes), 0) FROM snapshots`).Scan(&n)
	return n, err
}

func (s *Store) querySnapshots(q string, args ...any) ([]Snapshot, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Snapshot{}
	for rows.Next() {
		x, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
