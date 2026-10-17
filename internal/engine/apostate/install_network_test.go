// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build network

package apostate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/paths"
)

// Installs the pinned release of this OS (~150-190 MB) and verifies it end to end.
func TestInstallRealRelease(t *testing.T) {
	root, err := os.MkdirTemp("", "apnet")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	a := New(paths.Dirs{Root: root})
	ctx := context.Background()
	if st := a.Status(ctx, Version); st.Installed {
		t.Fatal("empty dir reported installed")
	}
	t0 := time.Now()
	if err := a.Install(ctx, Version, nil); err != nil {
		t.Fatal(err)
	}
	t.Logf("installed in %v", time.Since(t0).Round(time.Second))
	if st := a.Status(ctx, Version); !st.Installed || st.Problem != "" {
		t.Fatalf("status after install = %+v", st)
	}
	entries, _ := os.ReadDir(filepath.Dir(a.dir(Version)))
	if len(entries) != 1 || entries[0].Name() != Version {
		t.Fatalf("engines/apostate holds %v", entries)
	}
}
