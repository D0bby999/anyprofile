// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStorage(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	p, err := s.CreateProfile(ctx, NewProfile{Name: "Stored"})
	if err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(s.Dirs.Profile(p.ID), "Default", "Cache")
	if err := os.MkdirAll(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "f"), make([]byte, 1000), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Dirs.Profile(p.ID), "Default", "Cookies"), make([]byte, 10), 0o600); err != nil {
		t.Fatal(err)
	}
	zz := s.Dirs.Profile("zz")
	if err := os.MkdirAll(zz, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(zz, "a"), make([]byte, 500), 0o600); err != nil {
		t.Fatal(err)
	}

	info, err := s.Storage()
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Orphans) != 1 || info.Orphans[0].ID != "zz" || info.Orphans[0].Bytes != 500 {
		t.Fatalf("orphans = %+v", info.Orphans)
	}
	if info.ProfilesBytes < 1510 {
		t.Fatalf("profilesBytes = %d, want >= 1510", info.ProfilesBytes)
	}

	freed, err := s.ClearCache()
	if err != nil {
		t.Fatal(err)
	}
	if freed != 1000 {
		t.Fatalf("freed = %d, want 1000", freed)
	}
	if _, err := os.Stat(filepath.Join(s.Dirs.Profile(p.ID), "Default", "Cookies")); err != nil {
		t.Fatalf("cookies must survive: %v", err)
	}

	if err := s.DeleteOrphans([]string{"zz", p.ID, "../x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(zz); !os.IsNotExist(err) {
		t.Fatal("orphan zz survived")
	}
	if _, err := os.Stat(s.Dirs.Profile(p.ID)); err != nil {
		t.Fatal("known profile folder was deleted")
	}
}

// DeleteOrphans must only ever remove a folder the scan would list: never the
// profiles root itself, whatever id the caller sends.
func TestDeleteOrphansRefusesRoot(t *testing.T) {
	s, _ := newService(t)
	p, err := s.CreateProfile(context.Background(), NewProfile{Name: "Kept"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(s.Dirs.Profile(p.ID), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Dirs.Profiles(), "loose-file"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteOrphans([]string{"", ".", "..", "loose-file", `a\b`}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{s.Dirs.Profile(p.ID), filepath.Join(s.Dirs.Profiles(), "loose-file")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s was deleted: %v", path, err)
		}
	}
}
