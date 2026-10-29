// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestLegacyDefaultEngineBlocksMigration(t *testing.T) {
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
	if s != nil {
		s.Close()
	}
	if !errors.Is(err, ErrLegacyEngineMigrationBlocked) {
		t.Fatalf("Open error = %v", err)
	}
	old, err = openUpTo(path, dir, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	value, err := old.GetSetting("default_engine")
	if err != nil || value != "apostate" {
		t.Fatalf("default engine = %q, err = %v", value, err)
	}
}
