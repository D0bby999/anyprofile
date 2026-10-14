// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func snapshotProfile(t *testing.T) (*Service, string) {
	s, _ := newService(t)
	p, err := s.CreateProfile(context.Background(), NewProfile{Name: "Snap", Site: "other"})
	if err != nil {
		t.Fatal(err)
	}
	dir := s.Dirs.Profile(p.ID)
	if err := os.MkdirAll(filepath.Join(dir, "Default", "Cache"), 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "Default", "Cookies"), "v1")
	write(t, filepath.Join(dir, "Default", "Cache", "x"), "cache")
	return s, p.ID
}

func write(t *testing.T, p, v string) {
	if err := os.WriteFile(p, []byte(v), 0o600); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSnapshotRestoreAndUndo(t *testing.T) {
	s, id := snapshotProfile(t)
	cookies := filepath.Join(s.Dirs.Profile(id), "Default", "Cookies")
	snap, err := s.TakeSnapshot(id, "manual")
	if err != nil {
		t.Fatal(err)
	}
	write(t, cookies, "v2")
	before, err := s.RestoreSnapshot(snap.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := read(t, cookies); got != "v1" {
		t.Fatalf("after restore = %q", got)
	}
	if _, err := os.Stat(filepath.Join(s.Dirs.Profile(id), "Default", "Cache")); !os.IsNotExist(err) {
		t.Fatal("cache came back from the snapshot")
	}
	list, _ := s.Snapshots(id)
	if len(list) != 2 || list[0].Reason != "before-restore" || list[0].ID != before {
		t.Fatalf("snapshots = %+v", list)
	}
	if _, err := s.RestoreSnapshot(before); err != nil {
		t.Fatal(err)
	}
	if got := read(t, cookies); got != "v2" {
		t.Fatalf("after undo = %q", got)
	}
	if ids, _ := s.orphanIDs(); len(ids) != 0 {
		t.Fatalf("restore left orphans: %v", ids)
	}
}

func TestSnapshotOnCloseAndKeep(t *testing.T) {
	s, id := snapshotProfile(t)
	wait := func() { s.BeforeLaunch(id) } // returns once the close snapshot is written
	s.AfterExit(id, 30*time.Second)
	wait()
	if list, _ := s.Snapshots(id); len(list) != 0 {
		t.Fatalf("a 30 s session was saved: %d", len(list))
	}
	s.AfterExit(id, 5*time.Minute)
	wait()
	if list, _ := s.Snapshots(id); len(list) != 1 || list[0].Reason != "close" {
		t.Fatalf("after a 5 min session = %+v", list)
	}
	for i := 0; i < 6; i++ {
		s.AfterExit(id, 5*time.Minute)
		wait()
	}
	list, _ := s.Snapshots(id)
	files, _ := os.ReadDir(s.Dirs.ProfileSnapshots(id))
	if len(list) != 5 || len(files) != 5 {
		t.Fatalf("kept %d rows and %d files, want 5", len(list), len(files))
	}
	if err := s.SetSnapshotSettings(SnapshotSettings{Auto: false, Keep: 11}); err == nil || !strings.Contains(err.Error(), "bad_keep") {
		t.Fatalf("keep 11 err = %v", err)
	}
}

func TestOrphansIgnoreRestoreFolders(t *testing.T) {
	s, _ := newService(t)
	if err := os.MkdirAll(filepath.Join(s.Dirs.Profiles(), ".restore-x"), 0o700); err != nil {
		t.Fatal(err)
	}
	if ids, _ := s.orphanIDs(); len(ids) != 0 {
		t.Fatalf("orphans = %v", ids)
	}
}

func TestPurgeRemovesSnapshots(t *testing.T) {
	s, id := snapshotProfile(t)
	if _, err := s.TakeSnapshot(id, "manual"); err != nil {
		t.Fatal(err)
	}
	if err := s.Trash(id); err != nil {
		t.Fatal(err)
	}
	if err := s.Purge(id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.Dirs.ProfileSnapshots(id)); !os.IsNotExist(err) {
		t.Fatal("snapshot folder survived the purge")
	}
}
