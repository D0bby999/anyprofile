// SPDX-License-Identifier: AGPL-3.0-or-later

// Package store persists profiles, proxies and live launches in SQLite.
// It never holds a secret: proxy passwords and cookie keys live in package secrets.
package store

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

var ErrNotFound = errors.New("not found")

// ErrLegacyEngineMigrationBlocked prevents an irreversible hard cut-over until
// the old engine, profile data and rollback path have been verified together.
var ErrLegacyEngineMigrationBlocked = errors.New("legacy Apostate data requires a verified AnyBrowser migration; database was not migrated")

type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the database at path. Before applying a
// pending migration to an existing database it copies the file into backupDir.
func Open(path, backupDir string) (*Store, error) { return openUpTo(path, backupDir, math.MaxInt) }

// openUpTo applies migrations up to maxVer only. Tests use it to build a
// database as an older app left it, then upgrade it with Open.
func openUpTo(path, backupDir string, maxVer int) (*Store, error) {
	_, statErr := os.Stat(path)
	existed := statErr == nil
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // one writer; the app is the only client
	s := &Store{db: db}
	if err := s.migrate(path, backupDir, existed, maxVer); err != nil {
		db.Close()
		return nil, err
	}
	// The -wal and -shm files hold database pages too.
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Chmod(p, 0o600); err != nil && !os.IsNotExist(err) {
			db.Close()
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(path, backupDir string, existed bool, maxVer int) error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return err
	}
	var cur int
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&cur); err != nil {
		return err
	}
	names, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(names, func(i, j int) bool { return names[i].Name() < names[j].Name() })
	// Migration 21 is the dedicated Apostate -> AnyBrowser migration: it
	// rewrites profiles and templates, and migration 23 finishes the saved
	// default engine. Before 21 this is the only migration path, so a database
	// from a released Apostate build must be able to take it after verifying
	// that every legacy profile is the known upstream shape (0.5.1 with its
	// apostate persona). Anything else fails closed instead of being
	// relabeled.
	//
	// At schema 21 Apostate rows are post-option data: migration 21 already
	// ran, so they cannot be pre-option legacy rows and must fail closed.
	// Schema 22 reintroduces Apostate as an explicit choice; its rows are not
	// touched by this guard.
	//
	// sessions are launch history, not identity: they record which engine ran,
	// carry no seed/persona/origin, and stay readable after schema 22 registers
	// Apostate again. An old database with launch history must not be unable to
	// migrate forever because of it.
	if cur > 0 && cur < 22 && maxVer >= 21 && maxVer > cur {
		if cur < 21 {
			var unknown int
			if err := s.db.QueryRow(`SELECT COUNT(*) FROM profiles
				WHERE (engine = 'apostate' OR persona_id IN ('apostate-macos', 'apostate-windows'))
				  AND NOT (engine = 'apostate' AND engine_version = '0.5.1'
				           AND persona_id IN ('apostate-macos', 'apostate-windows'))`).Scan(&unknown); err != nil {
				return err
			}
			if unknown > 0 {
				return ErrLegacyEngineMigrationBlocked
			}
		} else {
			var legacy int
			if err := s.db.QueryRow(`SELECT COUNT(*) FROM profiles WHERE engine = 'apostate' OR persona_id IN ('apostate-macos', 'apostate-windows')`).Scan(&legacy); err != nil {
				return err
			}
			checks := []struct {
				version int
				query   string
			}{
				{2, `SELECT COUNT(*) FROM settings WHERE key = 'default_engine' AND value = 'apostate'`},
				{19, `SELECT COUNT(*) FROM templates WHERE engine = 'apostate'`},
			}
			for _, check := range checks {
				if cur >= check.version {
					var count int
					if err := s.db.QueryRow(check.query).Scan(&count); err != nil {
						return err
					}
					legacy += count
				}
			}
			if legacy > 0 {
				return ErrLegacyEngineMigrationBlocked
			}
		}
	}
	backedUp := false
	for i, f := range names {
		ver, err := strconv.Atoi(strings.SplitN(f.Name(), "_", 2)[0])
		if err != nil || ver != i+1 {
			return fmt.Errorf("migration %s: numbers must be contiguous from 1", f.Name())
		}
		if ver <= cur {
			continue
		}
		if ver > maxVer {
			break
		}
		if existed && !backedUp {
			if err := s.backup(path, backupDir, cur); err != nil {
				return fmt.Errorf("backup before migration: %w", err)
			}
			backedUp = true
		}
		body, err := migrationFS.ReadFile("migrations/" + f.Name())
		if err != nil {
			return err
		}
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", f.Name(), err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_version (version) VALUES (?)`, ver); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) backup(path, dir string, ver int) error {
	if _, err := s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return err
	}
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()
	name := fmt.Sprintf("anyprofile-v%d-%s.db", ver, time.Now().UTC().Format("20060102-150405"))
	dst, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	return dst.Close()
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }
