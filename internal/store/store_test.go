// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// insertV1Profile writes a profile with only the columns of migration 0001,
// so it works on a database at any schema version.
func insertV1Profile(t *testing.T, s *Store, id, kind, createdAt string) {
	t.Helper()
	_, err := s.db.Exec(`INSERT INTO profiles (id, name, engine, engine_version, persona_id, seed_hex,
		platform, timezone, languages, ui_locale, channel_kind, created_at, updated_at)
		VALUES (?, ?, 'fury', '0.3.5', 'macos-m4-1710x1112', '00112233aabbccdd', 'macos',
		'UTC', 'en-US,en', 'en-US', ?, ?, ?)`, id, "Old "+id, kind, createdAt, createdAt)
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
	if err != nil || p.Tags[0] != "tiktok" || p.Site != "other" {
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

func TestSitesMigration(t *testing.T) {
	s, _ := upgradeFrom(t, 2, func(o *Store) {
		insertV1Profile(t, o, "p1", "tiktok", "2026-10-01T00:00:00Z")
		insertV1Profile(t, o, "p2", "other", "2026-10-02T00:00:00Z")
	})
	if p, err := s.GetProfile("p1"); err != nil || p.Site != "tiktok" {
		t.Fatalf("p1 = %+v, %v", p, err)
	}
	if p, err := s.GetProfile("p2"); err != nil || p.Site != "other" {
		t.Fatalf("p2 = %+v, %v", p, err)
	}
	if cs, err := s.ListCustomSites(); err != nil || len(cs) != 0 {
		t.Fatalf("custom sites = %v, %v", cs, err)
	}
}

func TestCustomSites(t *testing.T) {
	s, _ := openTemp(t)
	c := &CustomSite{ID: "custom-aa", Name: "Tiki", Domains: []string{"tiki.vn"}, StartURL: "https://tiki.vn", Color: "#1A94FF"}
	if err := s.CreateCustomSite(c); err != nil {
		t.Fatal(err)
	}
	p := sample("a")
	p.Site = "custom-aa"
	if err := s.CreateProfile(p); err != nil {
		t.Fatal(err)
	}
	got, err := s.ListCustomSites()
	if err != nil || len(got) != 1 || len(got[0].Domains) != 1 || got[0].Domains[0] != "tiki.vn" {
		t.Fatalf("list = %+v, %v", got, err)
	}
	c.Name = "Tiki 2"
	if err := s.UpdateCustomSite(*c); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.ListCustomSites(); got[0].Name != "Tiki 2" {
		t.Fatalf("after update = %+v", got)
	}
	if err := s.DeleteCustomSite("custom-aa"); err != nil {
		t.Fatal(err)
	}
	if p, _ := s.GetProfile("a"); p.Site != "other" {
		t.Fatalf("profile not moved to other: %+v", p)
	}
	if err := s.DeleteCustomSite("custom-aa"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete err = %v", err)
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

func TestSerial(t *testing.T) {
	s, _ := openTemp(t)
	ids := []string{"p1", "p2", "p3"}
	for i, id := range ids {
		p := sample(id)
		if err := s.CreateProfile(p); err != nil {
			t.Fatal(err)
		}
		if p.Serial != i+1 {
			t.Fatalf("%s serial = %d, want %d", id, p.Serial, i+1)
		}
	}
	if err := s.SoftDelete("p2"); err != nil {
		t.Fatal(err)
	}
	p4 := sample("p4")
	if err := s.CreateProfile(p4); err != nil {
		t.Fatal(err)
	}
	if p4.Serial != 4 {
		t.Fatalf("p4 serial = %d, want 4", p4.Serial)
	}
}

func TestSerialMigration(t *testing.T) {
	s, _ := upgradeFrom(t, 3, func(o *Store) {
		insertV1Profile(t, o, "b", "other", "2026-10-02T00:00:00Z")
		insertV1Profile(t, o, "a", "other", "2026-10-01T00:00:00Z")
	})
	if p, err := s.GetProfile("a"); err != nil || p.Serial != 1 {
		t.Fatalf("a serial = %d, %v", p.Serial, err)
	}
	if p, err := s.GetProfile("b"); err != nil || p.Serial != 2 {
		t.Fatalf("b serial = %d, %v", p.Serial, err)
	}
}

func TestAccountColumns(t *testing.T) {
	s, _ := openTemp(t)
	p := sample("p1")
	p.Login = "me@example.com"
	if err := s.CreateProfile(p); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetProfile("p1"); got.Login != "me@example.com" || got.HasPassword || got.HasTOTP {
		t.Fatalf("after create = %+v", got)
	}
	// UpdateProfile must not touch the secret flags.
	p.HasPassword = true
	p.HasTOTP = true
	if err := s.UpdateProfile(*p); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetProfile("p1"); got.HasPassword || got.HasTOTP {
		t.Fatalf("UpdateProfile changed flags: %+v", got)
	}
	if err := s.SetSecretFlag("p1", "has_totp", true); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetProfile("p1"); !got.HasTOTP {
		t.Fatalf("flag not set: %+v", got)
	}
	if err := s.SetSecretFlag("p1", "x", true); err == nil {
		t.Fatal("unknown flag must fail")
	}
	// The search matches the login too.
	if got, _ := s.ListProfiles(ProfileFilter{Query: "me@example"}); len(got) != 1 {
		t.Fatalf("login search = %d rows", len(got))
	}
}

func TestAccountsMigration(t *testing.T) {
	s, _ := upgradeFrom(t, 4, func(o *Store) {
		insertV1Profile(t, o, "p1", "other", "2026-10-01T00:00:00Z")
	})
	p, err := s.GetProfile("p1")
	if err != nil {
		t.Fatal(err)
	}
	if p.Login != "" || p.HasPassword || p.HasTOTP {
		t.Fatalf("upgraded profile = %+v", p)
	}
}

func TestStartURLs(t *testing.T) {
	p := Profile{StartURL: "a\n\n b \n"}
	got := p.StartURLs()
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("StartURLs = %v", got)
	}
}

func TestProxyMeta(t *testing.T) {
	s, _ := openTemp(t)
	px := &Proxy{ID: "px1", Label: "us-1", Scheme: "http", Host: "gw.example", Port: 10001, Username: "u", Note: "n", ExpiresOn: "2026-12-31"}
	if err := s.CreateProxy(px); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetProxy("px1"); got.Note != "n" || got.ExpiresOn != "2026-12-31" {
		t.Fatalf("get = %+v", got)
	}
	if err := s.UpdateProxyMeta("px1", "new", "note2", "2026-11-01"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetProxy("px1"); got.Label != "new" || got.Note != "note2" || got.ExpiresOn != "2026-11-01" {
		t.Fatalf("after update = %+v", got)
	}
}

func TestProxyMetaMigration(t *testing.T) {
	s, _ := upgradeFrom(t, 5, func(o *Store) {
		if _, err := o.db.Exec(`INSERT INTO proxies (id, label, scheme, host, port, username, created_at)
			VALUES ('x1', 'x1', 'http', 'h', 1, '', '2026-10-01T00:00:00Z')`); err != nil {
			t.Fatal(err)
		}
	})
	p, err := s.GetProxy("x1")
	if err != nil {
		t.Fatal(err)
	}
	if p.Note != "" || p.ExpiresOn != "" {
		t.Fatalf("migrated proxy = %+v", p)
	}
}

func TestFingerprint(t *testing.T) {
	s, _ := openTemp(t)
	p := sample("p1")
	if err := s.CreateProfile(p); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveFingerprint("p1", "{b}", "{b}", "baseline", nil); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveFingerprint("p1", "", "{x}", "drift", []string{"canvas"}); err != nil {
		t.Fatal(err)
	}
	if b, _ := s.FingerprintBaseline("p1"); b != "{b}" {
		t.Fatalf("baseline = %q", b)
	}
	if got, _ := s.GetProfile("p1"); got.FPVerdict != "drift" || len(got.FPDiff) != 1 || got.FPDiff[0] != "canvas" {
		t.Fatalf("profile = %+v", got)
	}
	if err := s.ResetFingerprint("p1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetProfile("p1"); got.FPVerdict != "" || len(got.FPDiff) != 0 {
		t.Fatalf("after reset = %+v", got)
	}
}

func TestFingerprintMigration(t *testing.T) {
	s, _ := upgradeFrom(t, 6, func(o *Store) {
		insertV1Profile(t, o, "p1", "other", "2026-10-01T00:00:00Z")
	})
	p, err := s.GetProfile("p1")
	if err != nil {
		t.Fatal(err)
	}
	if p.FPVerdict != "" || len(p.FPDiff) != 0 {
		t.Fatalf("migrated profile = %+v", p)
	}
}

func TestEngineMetaMigration(t *testing.T) {
	s, _ := upgradeFrom(t, 7, func(o *Store) {
		insertV1Profile(t, o, "p1", "other", "2026-10-01T00:00:00Z")
	})
	p, err := s.GetProfile("p1")
	if err != nil {
		t.Fatal(err)
	}
	if p.EngineMeta != (EngineMeta{}) {
		t.Fatalf("migrated meta = %+v", p.EngineMeta)
	}
}

func TestEngineMetaIsPinnedAtCreation(t *testing.T) {
	s, _ := openTemp(t)
	p := sample("m1")
	p.EngineMeta = EngineMeta{HWC: 8, MemGiB: 16}
	if err := s.CreateProfile(p); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetProfile("m1")
	if err != nil || got.EngineMeta != (EngineMeta{HWC: 8, MemGiB: 16}) {
		t.Fatalf("meta = %+v %v", got.EngineMeta, err)
	}
	got.EngineMeta = EngineMeta{HWC: 1}
	if err := s.UpdateProfile(got); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetProfile("m1")
	if got.EngineMeta != (EngineMeta{HWC: 8, MemGiB: 16}) {
		t.Fatalf("meta after update = %+v", got.EngineMeta)
	}
}

func TestMachineLabel(t *testing.T) {
	cases := map[string]string{
		"":  "",
		"{": "",
		`{"webglRenderer":"ANGLE (Apple, ANGLE Metal Renderer: Apple M4 Pro, Unspecified Version)","screen":"1512x982"}`: "Apple M4 Pro · 1512×982",
		`{"webglRenderer":"Apple M2","screen":"1440x900"}`:                                                               "Apple M2 · 1440×900",
		`{"webglRenderer":"Apple M2"}`:                                                                                   "",
	}
	for in, want := range cases {
		if got := machineLabel(in); got != want {
			t.Errorf("machineLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestProxyLabelMigration(t *testing.T) {
	s, _ := upgradeFrom(t, 8, func(o *Store) {
		if _, err := o.db.Exec(`INSERT INTO proxies (id, label, scheme, host, port, username, created_at) VALUES
			('a', 'http://u@h:1', 'http', 'h', 1, 'u', '2026-10-01T00:00:00Z'),
			('b', 'Shop US', 'http', 'h', 2, 'u', '2026-10-01T00:00:00Z')`); err != nil {
			t.Fatal(err)
		}
	})
	for id, want := range map[string]string{"a": "h:1", "b": "Shop US"} {
		p, err := s.GetProxy(id)
		if err != nil || p.Label != want {
			t.Fatalf("proxy %s label = %q %v, want %q", id, p.Label, err, want)
		}
	}
}

func TestSessions(t *testing.T) {
	s, _ := upgradeFrom(t, 9, func(o *Store) {
		insertV1Profile(t, o, "p1", "other", "2026-10-01T00:00:00Z")
	})
	if err := s.CreateProfile(sample("p2")); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 52; i++ {
		if err := s.StartSession(&Session{ProfileID: "p1", Engine: "apostate", EngineVersion: "0.5.1"}); err != nil {
			t.Fatal(err)
		}
	}
	all, _ := s.Sessions("p1", 100)
	if len(all) != MaxSessions {
		t.Fatalf("kept %d sessions", len(all))
	}
	newest := all[0]
	if err := s.EndSession(newest.ID); err != nil {
		t.Fatal(err)
	}
	first, _ := s.Sessions("p1", 1)
	stopped := first[0].StoppedAt
	if stopped == "" {
		t.Fatal("not stopped")
	}
	if err := s.EndSession(newest.ID); err != nil {
		t.Fatal(err)
	}
	again, _ := s.Sessions("p1", 1)
	if again[0].StoppedAt != stopped {
		t.Fatal("second EndSession moved the stop time")
	}

	s.SetSessionExit(all[2].ID, "1.1.1.1", "US", "America/New_York", "")
	s.SetSessionExit(all[1].ID, "2.2.2.2", "DE", "Europe/Berlin", "")
	ex, err := s.LatestExits([]string{"p1", "p2"})
	if err != nil {
		t.Fatal(err)
	}
	if ex["p1"][0].ExitIP != "2.2.2.2" || ex["p1"][1].ExitIP != "1.1.1.1" || ex["p2"][0].ID != 0 {
		t.Fatalf("latest exits = %+v", ex)
	}

	if err := s.StartSession(&Session{ProfileID: "p2", Engine: "fury", EngineVersion: "0.3.5"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CloseStaleSessions(map[string]bool{"p1": true}); err != nil {
		t.Fatal(err)
	}
	if _, open, _ := s.OpenSession("p2"); open {
		t.Fatal("stale session of a stopped profile left open")
	}
	if _, open, _ := s.OpenSession("p1"); !open {
		t.Fatal("running profile lost its open session")
	}

	if err := s.Purge("p1"); err != nil {
		t.Fatal(err)
	}
	if left, _ := s.Sessions("p1", 100); len(left) != 0 {
		t.Fatalf("sessions survived the purge: %d", len(left))
	}
}

func TestSnapshotsMigrationAndPrune(t *testing.T) {
	s, _ := upgradeFrom(t, 10, func(o *Store) {
		insertV1Profile(t, o, "p1", "other", "2026-10-01T00:00:00Z")
	})
	for i := 0; i < 7; i++ {
		if err := s.AddSnapshot(&Snapshot{ProfileID: "p1", Reason: "close", Bytes: 10, File: "f"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.AddSnapshot(&Snapshot{ProfileID: "p1", Reason: "manual", Bytes: 5, File: "m"}); err != nil {
		t.Fatal(err)
	}
	old, err := s.SnapshotsToPrune("p1", "close", 5)
	if err != nil || len(old) != 2 || old[0].ID != 2 || old[1].ID != 1 {
		t.Fatalf("to prune = %+v %v", old, err)
	}
	if n, _ := s.SnapshotBytes(); n != 75 {
		t.Fatalf("bytes = %d", n)
	}
	if err := s.AddSnapshot(&Snapshot{ProfileID: "p1", Reason: "nope", File: "x"}); err == nil {
		t.Fatal("unknown reason accepted")
	}
	if err := s.Purge("p1"); err != nil {
		t.Fatal(err)
	}
	if left, _ := s.Snapshots("p1"); len(left) != 0 {
		t.Fatalf("snapshots survived the purge: %d", len(left))
	}
}

func TestBackupDaily(t *testing.T) {
	s, _ := openTemp(t)
	if err := s.CreateProfile(sample("b1")); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "anyprofile-v6.db"), []byte("migration backup"), 0o600)
	day := time.Date(2026, 10, 1, 9, 0, 0, 0, time.Local)
	for i := 0; i < 2; i++ {
		if err := s.BackupDaily(dir, day); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := filepath.Glob(filepath.Join(dir, "daily-*.db")); len(got) != 1 {
		t.Fatalf("two calls on one day made %d files", len(got))
	}
	for i := 1; i < 9; i++ {
		if err := s.BackupDaily(dir, day.AddDate(0, 0, i)); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := filepath.Glob(filepath.Join(dir, "daily-*.db"))
	if len(got) != 7 || filepath.Base(got[0]) != "daily-20261003.db" {
		t.Fatalf("kept %v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "anyprofile-v6.db")); err != nil {
		t.Fatal("other backups must stay")
	}
	b, err := Open(got[len(got)-1], t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if _, err := b.GetProfile("b1"); err != nil {
		t.Fatalf("backup misses the profile: %v", err)
	}
}
