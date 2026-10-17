// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func uploadsRig(t *testing.T) (dir, outside string) {
	t.Helper()
	base := t.TempDir()
	dir = filepath.Join(base, "uploads")
	os.MkdirAll(filepath.Join(dir, "clips"), 0o700)
	os.MkdirAll(filepath.Join(dir, ".hidden"), 0o700)
	os.WriteFile(filepath.Join(dir, "clip.mp4"), []byte("1234"), 0o600)
	os.WriteFile(filepath.Join(dir, "clips", "b.mp4"), []byte("12"), 0o600)
	os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("x"), 0o600)
	os.WriteFile(filepath.Join(dir, ".hidden", "c.mp4"), []byte("x"), 0o600)
	outside = filepath.Join(base, "secret.txt")
	os.WriteFile(outside, []byte("private"), 0o600)
	if err := os.Symlink(outside, filepath.Join(dir, "link.txt")); err != nil {
		t.Skip("no symlinks here:", err)
	}
	os.Symlink(base, filepath.Join(dir, "up"))
	return dir, outside
}

func TestListUploadsShowsOnlyPlainFiles(t *testing.T) {
	dir, _ := uploadsRig(t)
	us, err := ListUploads(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, u := range us {
		names = append(names, u.Name)
	}
	if strings.Join(names, ",") != "clip.mp4,clips/b.mp4" || us[0].Size != 4 {
		t.Fatalf("uploads = %+v", us)
	}
}

func TestResolveUploadsStaysInTheFolder(t *testing.T) {
	dir, outside := uploadsRig(t)
	got, err := ResolveUploads(dir, []string{"clip.mp4", "clips/b.mp4"})
	if err != nil || len(got) != 2 || filepath.Base(got[1]) != "b.mp4" || !filepath.IsAbs(got[0]) {
		t.Fatalf("resolve = %v, %v", got, err)
	}
	for _, bad := range []string{outside, "../secret.txt", "clips/../../secret.txt", "link.txt", "up/secret.txt",
		"clips", ".", "", "missing.mp4", `\secret.txt`, "C:secret.txt"} {
		if _, err := ResolveUploads(dir, []string{"clip.mp4", bad}); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
	if _, err := ResolveUploads(dir, nil); err == nil {
		t.Error("no files was accepted")
	}
	if _, err := ResolveUploads("", []string{"clip.mp4"}); err == nil {
		t.Error("no folder was accepted")
	}
}
