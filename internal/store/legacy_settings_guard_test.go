// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestLegacyDefaultEngineMigrates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.db")
	old, err := openUpTo(path, dir, 20)
	if err != nil {
		t.Fatal(err)
	}
	if err := old.SetSetting("default_engine", "apostate"); err != nil {
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
	if err := s.db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&version); err != nil || version != 26 {
		t.Fatalf("schema version = %d, err = %v", version, err)
	}
	value, err := s.GetSetting("default_engine")
	if err != nil || value != "anybrowser" {
		t.Fatalf("default engine = %q, err = %v", value, err)
	}
}

// At schema 21 the rename already ran, so an apostate default is post-option
// data; it must fail closed rather than be relabeled.
func TestPostOptionDefaultEngineBlocksMigration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.db")
	old, err := openUpTo(path, dir, 21)
	if err != nil {
		t.Fatal(err)
	}
	if err := old.SetSetting("default_engine", "apostate"); err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path, dir)
	if s != nil {
		s.Close()
	}
	if !errors.Is(err, ErrLegacyEngineMigrationBlocked) {
		t.Fatalf("Open error = %v", err)
	}
}
