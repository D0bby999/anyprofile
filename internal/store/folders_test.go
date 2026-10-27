// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func seedFolderTest(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "folders.db"), filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func mkProfile(t *testing.T, s *Store, id string) {
	t.Helper()
	p := &Profile{ID: id, Name: "Kenh " + id, Engine: "apostate", PersonaID: "macos-m4",
		SeedHex: "0123456789abcdef", Platform: "macos", Timezone: "Asia/Ho_Chi_Minh",
		Languages: "vi-VN,vi", UILocale: "vi-VN", Sites: []string{"tiktok"}}
	if err := s.CreateProfile(p); err != nil {
		t.Fatal(err)
	}
}

func TestFoldersMigrationLeavesOldProfilesUnfiled(t *testing.T) {
	s := seedFolderTest(t)
	mkProfile(t, s, "old1")
	p, err := s.GetProfile("old1")
	if err != nil {
		t.Fatal(err)
	}
	if p.FolderID != "" {
		t.Fatalf("migrated profile folder = %q, want unfiled", p.FolderID)
	}
}

func TestFolderNameUniqueness(t *testing.T) {
	s := seedFolderTest(t)
	if err := s.CreateFolder(&Folder{ID: "f1", Name: "Khách A"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateFolder(&Folder{ID: "f2", Name: "  khách a "}); !errors.Is(err, ErrFolderNameTaken) {
		t.Fatalf("duplicate name err = %v, want ErrFolderNameTaken", err)
	}
	if err := s.RenameFolder("f1", "KHÁCH A"); err != nil {
		t.Fatalf("renaming to own name in another case: %v", err)
	}
	if err := s.CreateFolder(&Folder{ID: "f3", Name: "Khách B"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameFolder("f3", "khách a"); !errors.Is(err, ErrFolderNameTaken) {
		t.Fatalf("rename onto taken name err = %v, want ErrFolderNameTaken", err)
	}
}

func TestDeleteFolderMovesProfilesAndKeepsTrashFolder(t *testing.T) {
	s := seedFolderTest(t)
	if err := s.CreateFolder(&Folder{ID: "fa", Name: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateFolder(&Folder{ID: "fb", Name: "B"}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"p1", "p2", "p3"} {
		mkProfile(t, s, id)
		if err := s.SetProfileFolder(id, "fa"); err != nil {
			t.Fatal(err)
		}
	}
	// Trash keeps the folder: delete the folder while p3 is trashed and
	// the restore must land in the move target, not nowhere.
	if err := s.SoftDelete("p3"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteFolder("fa", "fb"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"p1", "p2"} {
		p, err := s.GetProfile(id)
		if err != nil {
			t.Fatal(err)
		}
		if p.FolderID != "fb" {
			t.Fatalf("%s folder = %q, want fb after folder delete", id, p.FolderID)
		}
	}
	if err := s.Restore("p3"); err != nil {
		t.Fatal(err)
	}
	p3, err := s.GetProfile("p3")
	if err != nil {
		t.Fatal(err)
	}
	if p3.FolderID != "fb" {
		t.Fatalf("restored p3 folder = %q, want fb", p3.FolderID)
	}
	if _, err := s.GetFolder("fa"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted folder lookup err = %v, want ErrNotFound", err)
	}
}

func TestListProfilesFolderFilter(t *testing.T) {
	s := seedFolderTest(t)
	if err := s.CreateFolder(&Folder{ID: "fa", Name: "A"}); err != nil {
		t.Fatal(err)
	}
	mkProfile(t, s, "p1")
	mkProfile(t, s, "p2")
	mkProfile(t, s, "p3")
	if err := s.SetProfileFolder("p1", "fa"); err != nil {
		t.Fatal(err)
	}
	inFolder, err := s.ListProfiles(ProfileFilter{FolderID: "fa"})
	if err != nil {
		t.Fatal(err)
	}
	if len(inFolder) != 1 || inFolder[0].ID != "p1" {
		t.Fatalf("folder filter = %+v, want only p1", inFolder)
	}
	unfiled, err := s.ListProfiles(ProfileFilter{Unfiled: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(unfiled) != 2 {
		t.Fatalf("unfiled filter = %d profiles, want 2", len(unfiled))
	}
	counts, err := s.FolderCounts()
	if err != nil {
		t.Fatal(err)
	}
	if counts["fa"] != 1 || counts[""] != 2 {
		t.Fatalf("counts = %v, want fa:1 and unfiled:2", counts)
	}
}
