// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestLegacyEngineMigrationFailsClosed(t *testing.T) {
	cases := []struct {
		name    string
		version int
		seed    func(*testing.T, *Store)
	}{
		{"profile", 20, func(t *testing.T, s *Store) {
			insertV1Profile(t, s, "old", "other", "2026-01-01")
			if _, err := s.db.Exec(`UPDATE profiles SET engine='apostate', engine_version='0.5.1', persona_id='apostate-macos' WHERE id='old'`); err != nil {
				t.Fatal(err)
			}
		}},
		{"trashed profile", 20, func(t *testing.T, s *Store) {
			insertV1Profile(t, s, "old", "other", "2026-01-01")
			if _, err := s.db.Exec(`UPDATE profiles SET engine='apostate', deleted_at='2026-01-02' WHERE id='old'`); err != nil {
				t.Fatal(err)
			}
		}},
		{"legacy persona", 20, func(t *testing.T, s *Store) {
			insertV1Profile(t, s, "old", "other", "2026-01-01")
			if _, err := s.db.Exec(`UPDATE profiles SET persona_id='apostate-windows' WHERE id='old'`); err != nil {
				t.Fatal(err)
			}
		}},
		{"template", 20, func(t *testing.T, s *Store) {
			if _, err := s.db.Exec(`INSERT INTO templates(id,name,engine,created_at,updated_at) VALUES ('t','Old','apostate','2026-01-01','2026-01-01')`); err != nil {
				t.Fatal(err)
			}
		}},
		{"pre-option schema", 21, func(t *testing.T, s *Store) {
			insertV1Profile(t, s, "old", "other", "2026-01-01")
			if _, err := s.db.Exec(`UPDATE profiles SET engine='apostate', persona_id='apostate-macos' WHERE id='old'`); err != nil {
				t.Fatal(err)
			}
		}},
		{"old schema", 1, func(t *testing.T, s *Store) {
			insertV1Profile(t, s, "old", "other", "2026-01-01")
			if _, err := s.db.Exec(`UPDATE profiles SET engine='apostate' WHERE id='old'`); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "profiles.db")
			old, err := openUpTo(path, dir, tc.version)
			if err != nil {
				t.Fatal(err)
			}
			tc.seed(t, old)
			if err := old.Close(); err != nil {
				t.Fatal(err)
			}
			if s, err := Open(path, dir); !errors.Is(err, ErrLegacyEngineMigrationBlocked) {
				if s != nil {
					s.Close()
				}
				t.Fatalf("Open error = %v; want migration blocked", err)
			}
			// Reopen without newer migrations to confirm schema and rows remain intact.
			old, err = openUpTo(path, dir, tc.version)
			if err != nil {
				t.Fatal(err)
			}
			defer old.Close()
			var version int
			if err := old.db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&version); err != nil || version != tc.version {
				t.Fatalf("schema version = %d, err = %v", version, err)
			}
			var legacy int
			query := `SELECT COUNT(*) FROM profiles WHERE engine='apostate' OR persona_id IN ('apostate-macos','apostate-windows')`
			if tc.name == "template" {
				query = `SELECT COUNT(*) FROM templates WHERE engine='apostate'`
			}
			if err := old.db.QueryRow(query).Scan(&legacy); err != nil || legacy != 1 {
				t.Fatalf("legacy rows = %d, err = %v", legacy, err)
			}
		})
	}
}

func TestNoLegacyEngineMigrationStillOpens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.db")
	old, err := openUpTo(path, dir, 20)
	if err != nil {
		t.Fatal(err)
	}
	insertV1Profile(t, old, "fury", "other", "2026-01-01")
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.GetProfile("fury")
	if err != nil || p.Engine != "fury" {
		t.Fatalf("profile = %+v, err = %v", p, err)
	}
}
