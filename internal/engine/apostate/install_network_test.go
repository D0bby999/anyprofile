// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build network

package apostate

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/paths"
)

// Installs the pinned release (~158 MB) and verifies it end to end. With
// ANYPROFILE_APOSTATE_ZIP set to an already downloaded zip, the download is
// skipped and every other step still runs.
func TestInstallRealRelease(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "apnet")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	a := New(paths.Dirs{Root: root})
	ctx := context.Background()
	if st := a.Status(ctx); st.Installed {
		t.Fatal("empty dir reported installed")
	}
	t0 := time.Now()
	if local := os.Getenv("ANYPROFILE_APOSTATE_ZIP"); local != "" {
		staging := filepath.Join(filepath.Dir(a.dir()), ".staging-"+Version)
		if err := os.MkdirAll(staging, 0o700); err != nil {
			t.Fatal(err)
		}
		copyFile(t, local, filepath.Join(staging, zipName))
		if err := a.installFrom(filepath.Join(staging, zipName), staging); err != nil {
			t.Fatal(err)
		}
		os.RemoveAll(staging)
	} else if err := a.Install(ctx, nil); err != nil {
		t.Fatal(err)
	}
	t.Logf("installed in %v", time.Since(t0).Round(time.Second))
	if st := a.Status(ctx); !st.Installed || st.Problem != "" {
		t.Fatalf("status after install = %+v", st)
	}
	entries, _ := os.ReadDir(filepath.Dir(a.dir()))
	if len(entries) != 1 || entries[0].Name() != Version {
		t.Fatalf("engines/apostate holds %v", entries)
	}
}

func copyFile(t *testing.T, src, dst string) {
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
	out.Close()
}
