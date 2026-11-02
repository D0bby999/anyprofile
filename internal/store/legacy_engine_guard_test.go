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
			if err := old.db.QueryRow(query).Scan(&legacy); err != nil || legacy != 1 {
				t.Fatalf("legacy rows = %d, err = %v", legacy, err)
			}
		})
	}
}

// A database from a released Apostate build is exactly what migration 21 was
// written for: the known 0.5.1 profile becomes AnyBrowser with the persona
// renamed, and templates follow. The guard must not strand those users.
func TestKnownLegacyEngineMigrates(t *testing.T) {
	for _, kind := range []string{"profile", "template"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "profiles.db")
			old, err := openUpTo(path, dir, 20)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "profile" {
				insertV1Profile(t, old, "old", "other", "2026-01-01")
				if _, err := old.db.Exec(`UPDATE profiles SET engine='apostate', engine_version='0.5.1',
					persona_id='apostate-macos' WHERE id='old'`); err != nil {
					t.Fatal(err)
				}
			} else if _, err := old.db.Exec(`INSERT INTO templates(id,name,engine,created_at,updated_at)
				VALUES ('t','Old','apostate','2026-01-01','2026-01-01')`); err != nil {
				t.Fatal(err)
			}
			if err := old.Close(); err != nil {
				t.Fatal(err)
			}
			s, err := Open(path, dir)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			var version int
			if err := s.db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&version); err != nil || version != 27 {
				t.Fatalf("schema version = %d, err = %v", version, err)
			}
			if kind == "profile" {
				p, err := s.GetProfile("old")
				if err != nil || p.Engine != "anybrowser" || p.EngineVersion != "0.5.1.156" ||
					p.PersonaID != "anybrowser-macos" || p.SeedHex != "00112233aabbccdd" {
					t.Fatalf("profile = %+v, err = %v", p, err)
				}
			} else {
				var engine string
				if err := s.db.QueryRow(`SELECT engine FROM templates WHERE id='t'`).Scan(&engine); err != nil || engine != "anybrowser" {
					t.Fatalf("template engine = %q, err = %v", engine, err)
				}
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

// Launch history names the engine that ran; it carries no profile identity.
// It must not make a database impossible to migrate after Apostate became an
// explicit option again (schema 22).
func TestLegacySessionHistoryDoesNotBlockMigration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.db")
	old, err := openUpTo(path, dir, 20)
	if err != nil {
		t.Fatal(err)
	}
	insertV1Profile(t, old, "old", "other", "2026-01-01")
	if _, err := old.db.Exec(`INSERT INTO sessions(profile_id, engine, engine_version, started_at)
		VALUES ('old', 'apostate', '0.5.1', '2026-01-01')`); err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var version, sessions int
	if err := s.db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&version); err != nil || version != 27 {
		t.Fatalf("schema version = %d, err = %v", version, err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE engine = 'apostate'`).Scan(&sessions); err != nil || sessions != 1 {
		t.Fatalf("old session history = %d, err = %v", sessions, err)
	}
}
