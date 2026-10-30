// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestApostateOptionSurvivesReopenAndMerge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.db")
	s, err := Open(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	p := sample("upstream")
	p.Engine, p.EngineVersion, p.PersonaID = "apostate", "0.5.1", "apostate-macos"
	if err := s.CreateProfile(p); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO templates(id,name,engine,created_at,updated_at) VALUES ('t','Upstream','apostate','2026-01-01','2026-01-01')`); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting("default_engine", "anybrowser"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.GetProfile(p.ID)
	if err != nil || got.Engine != p.Engine || got.PersonaID != p.PersonaID || got.SeedHex != p.SeedHex {
		t.Fatalf("reopened profile = %+v, error = %v", got, err)
	}
	if value, err := s.GetSetting("default_engine"); err != nil || value != "anybrowser" {
		t.Fatalf("default = %q, error = %v", value, err)
	}
	backup := filepath.Join(dir, "backup.db")
	if err := s.CopyTo(backup); err != nil {
		t.Fatal(err)
	}
	dst, _ := openTemp(t)
	res, err := dst.Merge(context.Background(), backup, nil)
	if err != nil || len(res.Profiles) != 1 || len(res.Templates) != 1 {
		t.Fatalf("merge = %+v, error = %v", res, err)
	}
	got, err = dst.GetProfile(p.ID)
	if err != nil || got.Engine != "apostate" || got.PersonaID != p.PersonaID || got.SeedHex != p.SeedHex {
		t.Fatalf("merged profile = %+v, error = %v", got, err)
	}
}

func TestApostateOptionMigrationPreservesExistingData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.db")
	s, err := openUpTo(path, dir, 21)
	if err != nil {
		t.Fatal(err)
	}
	p := sample("existing")
	p.Engine, p.PersonaID = "anybrowser", "anybrowser-macos"
	if err := s.CreateProfile(p); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting("default_engine", "anybrowser"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.GetProfile(p.ID)
	if err != nil || got.Engine != p.Engine || got.PersonaID != p.PersonaID || got.SeedHex != p.SeedHex {
		t.Fatalf("profile changed during migration: %+v, error = %v", got, err)
	}
	if value, err := s.GetSetting("default_engine"); err != nil || value != "anybrowser" {
		t.Fatalf("default changed: %q, error = %v", value, err)
	}
}
