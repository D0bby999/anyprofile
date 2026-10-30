// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestMergeRejectsLegacyIdentityBeforeSideEffects(t *testing.T) {
	for _, kind := range []string{"profile", "template"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			src, err := openUpTo(filepath.Join(dir, "legacy.db"), dir, 21)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { src.Close() })
			if kind == "profile" {
				p := sample("legacy")
				p.Engine = "apostate"
				if err := src.CreateProfile(p); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := src.db.Exec(`INSERT INTO templates(id,name,engine,created_at,updated_at) VALUES ('t','Old','apostate','2026-01-01','2026-01-01')`); err != nil {
					t.Fatal(err)
				}
			}
			backup := filepath.Join(t.TempDir(), "backup.db")
			if err := src.CopyTo(backup); err != nil {
				t.Fatal(err)
			}
			dst, _ := openTemp(t)
			called := false
			_, err = dst.Merge(context.Background(), backup, func(MergeResult) error { called = true; return nil })
			if !errors.Is(err, ErrLegacyEngineMigrationBlocked) || called {
				t.Fatalf("error = %v, callback = %v", err, called)
			}
			if _, err := dst.GetProfile("legacy"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("legacy imported: %v", err)
			}
		})
	}
}
