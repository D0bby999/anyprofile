// SPDX-License-Identifier: AGPL-3.0-or-later

package engine_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/engine/anybrowser"
)

// TestPinnedEngineArchive checks a downloaded engine archive the way install
// does: unpack it, hash the files the pin names, then verify the bundle's
// signature and team. ENGINE_ZIP points at the archive to check.
func TestPinnedEngineArchive(t *testing.T) {
	zipPath := os.Getenv("ENGINE_ZIP")
	if zipPath == "" {
		t.Skip("set ENGINE_ZIP to an engine archive")
	}
	rels := anybrowser.Builtin()
	if len(rels) != 1 {
		t.Fatalf("builtin releases: %d", len(rels))
	}
	rel := rels[0]

	dir := t.TempDir()
	if err := engine.ExtractZip(zipPath, dir); err != nil {
		t.Fatalf("extract: %v", err)
	}
	for name, want := range rel.Files {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("open %s: %v", name, err)
		}
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			f.Close()
			t.Fatalf("hash %s: %v", name, err)
		}
		f.Close()
		if got := hex.EncodeToString(h.Sum(nil)); got != want {
			t.Errorf("%s: sha256 %s, pin %s", filepath.Base(name), got, want)
		}
	}
	if err := engine.VerifyCodesign(filepath.Join(dir, rel.App), rel.TeamID); err != nil {
		t.Fatalf("codesign: %v", err)
	}
	t.Logf("ok: %s %s team %s", rel.Engine, rel.Version, rel.TeamID)
}
