// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package anybrowser

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/paths"
)

// TestComposeIdentityDrawsAServableSeed runs the real binary's explain mode on
// a throwaway tree. The seed it returns must be servable, and the machine it
// pins must not claim more cores than this host has.
func TestComposeIdentityDrawsAServableSeed(t *testing.T) {
	app := os.Getenv("ANYPROFILE_TEST_ANYBROWSER_APP")
	if app == "" {
		t.Skip("set ANYPROFILE_TEST_ANYBROWSER_APP to an extracted AnyBrowser app")
	}
	root, err := os.MkdirTemp("", "compose-integration-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	dirs := paths.Dirs{Root: root}
	if err := dirs.Ensure(); err != nil {
		t.Fatal(err)
	}
	a := New(dirs)
	if err := os.MkdirAll(filepath.Dir(a.AppPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(app, a.AppPath()); err != nil {
		t.Fatal(err)
	}
	id, err := a.ComposeIdentity(context.Background(), a.pinned.Version, 0)
	if err != nil {
		t.Fatal(err)
	}
	if id.SeedHex == "" {
		t.Fatal("no seed drawn; the engine could not be run")
	}
	if id.Meta.HWC <= 0 || id.Meta.MemGiB <= 0 {
		t.Fatalf("pinned cores=%d mem=%d, want a fully composed machine", id.Meta.HWC, id.Meta.MemGiB)
	}
	comp, err := a.compose(context.Background(), a.pinned.Version, id.SeedHex)
	if err != nil {
		t.Fatal(err)
	}
	if !comp.Servable || comp.Cores != id.Meta.HWC {
		t.Fatalf("seed %s composes %+v, pinned %d cores", id.SeedHex, comp, id.Meta.HWC)
	}
	cores, _ := a.hostInfo()
	if id.Meta.HWC > cores {
		t.Fatalf("pinned %d cores on a %d-core host", id.Meta.HWC, cores)
	}
	t.Logf("seed=%s cores=%d mem=%dGiB host=%d", id.SeedHex, id.Meta.HWC, id.Meta.MemGiB, cores)
}

// TestComposeIdentityBulk checks that many profiles in a row each get a
// servable seed and that the explain runs leave no temporary profile behind.
func TestComposeIdentityBulk(t *testing.T) {
	app := os.Getenv("ANYPROFILE_TEST_ANYBROWSER_APP")
	if app == "" {
		t.Skip("set ANYPROFILE_TEST_ANYBROWSER_APP to an extracted AnyBrowser app")
	}
	root, err := os.MkdirTemp("", "compose-bulk-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	dirs := paths.Dirs{Root: root}
	if err := dirs.Ensure(); err != nil {
		t.Fatal(err)
	}
	a := New(dirs)
	if err := os.MkdirAll(filepath.Dir(a.AppPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(app, a.AppPath()); err != nil {
		t.Fatal(err)
	}
	before := composeTempDirs(t)
	cores, mem := a.hostInfo()
	started := time.Now()
	for i := 0; i < 10; i++ {
		id, err := a.ComposeIdentity(context.Background(), a.pinned.Version, 0)
		if err != nil {
			t.Fatalf("draw %d: %v", i, err)
		}
		if id.SeedHex == "" {
			t.Fatalf("draw %d: no seed", i)
		}
		if id.Meta.HWC <= 0 || id.Meta.HWC > cores || id.Meta.MemGiB <= 0 || id.Meta.MemGiB > mem {
			t.Fatalf("draw %d: cores=%d mem=%d on a %d-core/%dGiB host", i, id.Meta.HWC, id.Meta.MemGiB, cores, mem)
		}
	}
	elapsed := time.Since(started)
	// Every explain run removes its profile; a leak would show up as new dirs.
	if leftover := composeTempDirs(t).Sub(before); len(leftover) > 0 {
		t.Fatalf("explain left temporary profiles behind: %v", leftover)
	}
	if elapsed > 30*time.Second {
		t.Fatalf("10 draws took %s, want well under 30s", elapsed)
	}
	t.Logf("10 draws in %s", elapsed.Round(time.Millisecond))
}

func composeTempDirs(t *testing.T) stringSet {
	t.Helper()
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	out := stringSet{}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "anyprofile-compose-") {
			out[e.Name()] = struct{}{}
		}
	}
	return out
}

type stringSet map[string]struct{}

func (s stringSet) Sub(other stringSet) []string {
	var out []string
	for name := range s {
		if _, ok := other[name]; !ok {
			out = append(out, name)
		}
	}
	return out
}
