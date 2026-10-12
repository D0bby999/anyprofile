// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build network

package fury

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/paths"
)

// Downloads the real pinned release (~104 MB) and verifies it end to end.
func TestInstallRealRelease(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "apnet")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	f := New(paths.Dirs{Root: root}, "/nonexistent")
	if st := f.Status(context.Background()); st.Installed {
		t.Fatal("empty dir reported installed")
	}
	t0 := time.Now()
	var last int64
	if err := f.Install(context.Background(), func(d, _ int64) { last = d }); err != nil {
		t.Fatal(err)
	}
	t.Logf("installed in %v, %d MB downloaded", time.Since(t0).Round(time.Second), last>>20)
	if st := f.Status(context.Background()); !st.Installed || st.Problem != "" {
		t.Fatalf("status after install = %+v", st)
	}
	t1 := time.Now()
	if st := f.Status(context.Background()); !st.Installed {
		t.Fatal("second status check failed")
	}
	t.Logf("status check with stamp: %v", time.Since(t1))
	if _, err := os.Stat(filepath.Join(f.dir(), "..", tarballName)); !os.IsNotExist(err) {
		t.Fatal("tarball left behind")
	}
}
