// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

func TestMergeFoldersTemplatesAndNameCollision(t *testing.T) {
	for _, collision := range []bool{false, true} {
		t.Run(fmt.Sprint(collision), func(t *testing.T) {
			src := seedFolderTest(t)
			if err := src.CreateFolder(&Folder{ID: "source-folder", Name: "Khách A", Color: "blue"}); err != nil {
				t.Fatal(err)
			}
			mkProfile(t, src, "profile-a")
			if err := src.SetProfileFolder("profile-a", "source-folder"); err != nil {
				t.Fatal(err)
			}
			if err := src.CreateTemplate(&Template{ID: "tpl", Name: "Template", FolderID: "source-folder", Sites: []string{"tiktok"}}); err != nil {
				t.Fatal(err)
			}
			bak := filepath.Join(t.TempDir(), "bak.db")
			if err := src.CopyTo(bak); err != nil {
				t.Fatal(err)
			}
			dst := seedFolderTest(t)
			want := "source-folder"
			if collision {
				want = "local-folder"
				if err := dst.CreateFolder(&Folder{ID: want, Name: "KHÁCH A"}); err != nil {
					t.Fatal(err)
				}
			}
			res, err := dst.Merge(context.Background(), bak, nil)
			if err != nil {
				t.Fatal(err)
			}
			p, err := dst.GetProfile("profile-a")
			if err != nil || p.FolderID != want {
				t.Fatalf("profile=%+v err=%v", p, err)
			}
			tpl, err := dst.GetTemplate("tpl")
			if err != nil || tpl.FolderID != want {
				t.Fatalf("template=%+v err=%v", tpl, err)
			}
			fs, err := dst.ListFolders()
			if err != nil || len(fs) != 1 {
				t.Fatalf("folders=%+v err=%v", fs, err)
			}
			if len(res.Templates) != 1 {
				t.Fatalf("merge result=%+v", res)
			}
			if _, err := dst.Merge(context.Background(), bak, nil); err != nil {
				t.Fatalf("repeat merge=%v", err)
			}
		})
	}
}

func TestMergeOrganizationRollback(t *testing.T) {
	src := seedFolderTest(t)
	if err := src.CreateFolder(&Folder{ID: "f", Name: "F"}); err != nil {
		t.Fatal(err)
	}
	if err := src.CreateTemplate(&Template{ID: "t", Name: "T", FolderID: "f"}); err != nil {
		t.Fatal(err)
	}
	bak := filepath.Join(t.TempDir(), "bak.db")
	if err := src.CopyTo(bak); err != nil {
		t.Fatal(err)
	}
	dst := seedFolderTest(t)
	boom := errors.New("rollback")
	if _, err := dst.Merge(context.Background(), bak, func(MergeResult) error { return boom }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if _, err := dst.GetFolder("f"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("folder survived rollback: %v", err)
	}
	if _, err := dst.GetTemplate("t"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("template survived rollback: %v", err)
	}
}

func TestReorderFoldersKeepsUnlistedAfterListed(t *testing.T) {
	s := seedFolderTest(t)
	for _, id := range []string{"a", "b", "c"} {
		if err := s.CreateFolder(&Folder{ID: id, Name: id}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ReorderFolders([]string{"c", "b"}); err != nil {
		t.Fatal(err)
	}
	fs, err := s.ListFolders()
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{"c", "b", "a"} {
		if fs[i].ID != id {
			t.Fatalf("order=%+v", fs)
		}
	}
	for _, ids := range [][]string{{"c", "c"}, {"missing"}} {
		if err := s.ReorderFolders(ids); err == nil {
			t.Fatalf("accepted invalid order %v", ids)
		}
	}
	fs, _ = s.ListFolders()
	if fs[0].ID != "c" || fs[1].ID != "b" {
		t.Fatal("failed reorder changed order")
	}
}

func TestConcurrentFolderNamesRemainUnique(t *testing.T) {
	s := seedFolderTest(t)
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); errs <- s.CreateFolder(&Folder{ID: fmt.Sprint(i), Name: "Khách A"}) }(i)
	}
	wg.Wait()
	close(errs)
	created := 0
	for err := range errs {
		if err == nil {
			created++
		} else if !errors.Is(err, ErrFolderNameTaken) {
			t.Fatal(err)
		}
	}
	if created != 1 {
		t.Fatalf("created=%d, want exactly 1", created)
	}
}

func TestSetProfilesFolderAtomicAndDeduplicated(t *testing.T) {
	s := seedFolderTest(t)
	if err := s.CreateFolder(&Folder{ID: "f", Name: "F"}); err != nil {
		t.Fatal(err)
	}
	mkProfile(t, s, "p1")
	mkProfile(t, s, "p2")
	if n, err := s.SetProfilesFolder([]string{"p1", "missing"}, "f"); err == nil || n != 0 {
		t.Fatalf("partial batch: n=%d err=%v", n, err)
	}
	p, err := s.GetProfile("p1")
	if err != nil || p.FolderID != "" {
		t.Fatalf("partial move survived: %+v %v", p, err)
	}
	if n, err := s.SetProfilesFolder([]string{"p1", "p1", "p2"}, "f"); err != nil || n != 2 {
		t.Fatalf("batch: n=%d err=%v", n, err)
	}
	if n, err := s.SetProfilesFolder([]string{"p1"}, "gone"); err == nil || n != 0 {
		t.Fatalf("missing destination: n=%d err=%v", n, err)
	}
}
