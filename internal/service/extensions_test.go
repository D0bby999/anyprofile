// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestExtensions(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "manifest.json"), []byte(`{"manifest_version": 3, "name": "Helper", "version": "1.0"}`), 0o600)
	e, err := s.AddExtension(src)
	if err != nil || e.Name != "Helper" || !strings.HasPrefix(e.ID, "ext-") {
		t.Fatalf("add = %+v, %v", e, err)
	}
	if _, err := os.Stat(filepath.Join(s.Dirs.Extension(e.ID), "manifest.json")); err != nil {
		t.Fatalf("not unpacked: %v", err)
	}
	if _, err := s.AddExtension(t.TempDir()); err == nil || !strings.Contains(err.Error(), "extension_no_manifest") {
		t.Fatalf("empty folder err = %v", err)
	}

	a, _ := s.CreateProfile(ctx, NewProfile{Name: "A"})
	b, _ := s.CreateProfile(ctx, NewProfile{Name: "B"})
	if err := s.SetExtensionMany([]string{a.ID, b.ID}, e.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetExtensionMany([]string{b.ID}, e.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := s.SetExtensionMany([]string{a.ID}, "ext-nope", true); err == nil || !strings.Contains(err.Error(), "extension_unknown") {
		t.Fatalf("unknown extension err = %v", err)
	}
	pa, _ := s.Store.GetProfile(a.ID)
	pb, _ := s.Store.GetProfile(b.ID)
	if !slices.Equal(pa.Extensions, []string{e.ID}) || len(pb.Extensions) != 0 {
		t.Fatalf("a = %v, b = %v", pa.Extensions, pb.Extensions)
	}
	if list, _ := s.ListExtensions(); len(list) != 1 || list[0].Profiles != 1 {
		t.Fatalf("list = %+v", list)
	}
	dups, err := s.DuplicateProfile(ctx, DuplicateInput{SourceID: a.ID, Count: 1, Proxy: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if d, _ := s.Store.GetProfile(dups[0].ID); !slices.Equal(d.Extensions, []string{e.ID}) {
		t.Fatalf("duplicate lost the extension: %v", d.Extensions)
	}

	if err := s.DeleteExtension(e.ID); err != nil {
		t.Fatal(err)
	}
	if pa, _ := s.Store.GetProfile(a.ID); len(pa.Extensions) != 0 {
		t.Fatalf("profile still loads a deleted extension: %v", pa.Extensions)
	}
	if _, err := os.Stat(s.Dirs.Extension(e.ID)); !os.IsNotExist(err) {
		t.Fatalf("folder left: %v", err)
	}
}

func TestImportBookmarks(t *testing.T) {
	s, _ := newService(t)
	p, _ := s.CreateProfile(context.Background(), NewProfile{Name: "A"})
	f := filepath.Join(t.TempDir(), "b.html")
	os.WriteFile(f, []byte(`<DL><p><DT><A HREF="https://example.com/">Example</A><DT><A HREF="https://example.org/">Org</A></DL>`), 0o600)
	n, err := s.ImportBookmarks(p.ID, f, "Imported")
	if err != nil || n != 2 {
		t.Fatalf("import = %d, %v", n, err)
	}
	b, err := os.ReadFile(filepath.Join(s.Dirs.Profile(p.ID), "Default", "Bookmarks"))
	if err != nil || !strings.Contains(string(b), "https://example.org/") {
		t.Fatalf("Bookmarks = %s, %v", b, err)
	}
	empty := filepath.Join(t.TempDir(), "e.html")
	os.WriteFile(empty, []byte(`<DL></DL>`), 0o600)
	if _, err := s.ImportBookmarks(p.ID, empty, "Imported"); err == nil || !strings.Contains(err.Error(), "bookmarks_unreadable") {
		t.Fatalf("empty file err = %v", err)
	}
}
