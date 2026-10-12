// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// insertV1Profile writes a profile with only the columns of migration 0001,
// so it works on a database at any schema version.
func insertV1Profile(t *testing.T, s *Store, id, channelKind, createdAt string) {
	t.Helper()
	_, err := s.db.Exec(`INSERT INTO profiles (id, name, engine, engine_version, persona_id, seed_hex,
		platform, timezone, languages, ui_locale, channel_kind, created_at, updated_at)
		VALUES (?, ?, 'fury', '0.3.5', 'macos-m4-1710x1112', '00112233aabbccdd', 'macos',
		'UTC', 'en-US,en', 'en-US', ?, ?, ?)`, id, "Old "+id, channelKind, createdAt, createdAt)
	if err != nil {
		t.Fatal(err)
	}
}

// upgradeFrom builds a database at version ver, lets seed fill it, then
// reopens it with Open so every later migration runs as in a real upgrade.
func upgradeFrom(t *testing.T, ver int, seed func(s *Store)) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "anyprofile.db")
	old, err := openUpTo(path, dir, ver)
	if err != nil {
		t.Fatal(err)
	}
	seed(old)
	old.Close()
	s, err := Open(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, dir
}

func TestSettingsMigrationAndRoundTrip(t *testing.T) {
	s, dir := upgradeFrom(t, 1, func(o *Store) {
		insertV1Profile(t, o, "p1", "other", "2026-10-01T00:00:00Z")
	})
	if p, err := s.GetProfile("p1"); err != nil || p.ID != "p1" {
		t.Fatalf("GetProfile after upgrade = %+v, %v", p, err)
	}
	if b, _ := filepath.Glob(filepath.Join(dir, "anyprofile-v1-*.db")); len(b) != 1 {
		t.Fatalf("backup files = %v", b)
	}
	if err := s.SetSetting("language", "en"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting("language", "vi"); err != nil {
		t.Fatal(err)
	}
	if v, err := s.GetSetting("language"); err != nil || v != "vi" {
		t.Fatalf("GetSetting(language) = %q, %v", v, err)
	}
	if v, err := s.GetSetting("missing"); err != nil || v != "" {
		t.Fatalf("GetSetting(missing) = %q, %v", v, err)
	}
}

func openTemp(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "anyprofile.db")
	s, err := Open(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func sample(id string) *Profile {
	return &Profile{ID: id, Name: "Kenh " + id, Engine: "fury", EngineVersion: "0.3.5",
		PersonaID: "macos-m4-1710x1112", SeedHex: "00112233aabbccdd", Platform: "macos",
		Timezone: "America/New_York", Languages: "en-US,en", UILocale: "en-US", Tags: []string{"tiktok"}}
}

func TestOpenSetsModeAndIsIdempotent(t *testing.T) {
	s, path := openTemp(t)
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("db mode = %v", st.Mode().Perm())
	}
	// A world-readable WAL left by an older build is tightened on open.
	os.Chmod(path+"-wal", 0o644)
	s.Close()
	os.WriteFile(path+"-wal", nil, 0o644)
	if s2, err := Open(path, filepath.Dir(path)); err == nil {
		s2.Close()
	}
	if st, err := os.Stat(path + "-wal"); err == nil && st.Mode().Perm() != 0o600 {
		t.Fatalf("wal mode = %v", st.Mode().Perm())
	}
	s2, err := Open(path, filepath.Dir(path))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	s2.Close()
	if b, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "anyprofile-v*.db")); len(b) != 0 {
		t.Fatalf("no migration pending, yet backups were made: %v", b)
	}
}

func TestProfileLifecycle(t *testing.T) {
	s, _ := openTemp(t)
	if err := s.CreateProfile(sample("a")); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateProfile(sample("b")); err != nil {
		t.Fatal(err)
	}
	p, err := s.GetProfile("a")
	if err != nil || p.Tags[0] != "tiktok" || p.ChannelKind != "other" {
		t.Fatalf("get = %+v, %v", p, err)
	}

	p.Name, p.Notes = "Renamed", "note"
	if err := s.UpdateProfile(p); err != nil {
		t.Fatal(err)
	}
	bad := p
	bad.SeedHex = "ffffffffffffffff"
	if err := s.UpdateProfile(bad); err == nil {
		t.Fatal("changing the seed must be refused")
	}

	if got, _ := s.ListProfiles(ProfileFilter{Query: "renam"}); len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("query filter = %+v", got)
	}
	if got, _ := s.ListProfiles(ProfileFilter{Tag: "tiktok"}); len(got) != 2 {
		t.Fatalf("tag filter = %d", len(got))
	}

	if err := s.SoftDelete("a"); err != nil {
		t.Fatal(err)
	}
	if live, _ := s.ListProfiles(ProfileFilter{}); len(live) != 1 {
		t.Fatalf("live after delete = %d", len(live))
	}
	if trash, _ := s.ListProfiles(ProfileFilter{Trashed: true}); len(trash) != 1 || trash[0].ID != "a" {
		t.Fatalf("trash = %+v", trash)
	}
	if err := s.Restore("a"); err != nil {
		t.Fatal(err)
	}
	if err := s.Purge("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetProfile("a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after purge err = %v", err)
	}
}

func TestProxyDetachAndLaunches(t *testing.T) {
	s, _ := openTemp(t)
	px := &Proxy{ID: "px1", Label: "us-1", Scheme: "http", Host: "gw.example", Port: 10001, Username: "u"}
	if err := s.CreateProxy(px); err != nil {
		t.Fatal(err)
	}
	p := sample("a")
	p.ProxyID = "px1"
	if err := s.CreateProfile(p); err != nil {
		t.Fatal(err)
	}
	if use, _ := s.ProxyUsage(); len(use["px1"]) != 1 {
		t.Fatalf("usage = %v", use)
	}
	if err := s.DeleteProxy("px1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetProfile("a"); got.ProxyID != "" {
		t.Fatalf("proxy not detached: %q", got.ProxyID)
	}

	if err := s.PutLaunch(Launch{ProfileID: "a", PID: 42, PIDStartTime: "t"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PutLaunch(Launch{ProfileID: "a", PID: 43, PIDStartTime: "t2"}); err != nil {
		t.Fatal(err)
	}
	ls, _ := s.ListLaunches()
	if len(ls) != 1 || ls[0].PID != 43 {
		t.Fatalf("launches = %+v", ls)
	}
	if err := s.Purge("a"); err != nil {
		t.Fatal(err)
	}
	if ls, _ := s.ListLaunches(); len(ls) != 0 {
		t.Fatalf("launch survived purge: %+v", ls)
	}
}
