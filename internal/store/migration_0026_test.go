// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import "testing"

// Migration 0026 drops the baseline that migration 0024 left measuring the
// Apostate build: an AnyBrowser profile cannot have been measured on Chromium
// 155, and keeping that baseline made every launch report drift.
func TestStaleFingerprintBaselineAfterEngineRebuild(t *testing.T) {
	const ua155 = `{"canvas":"4b92cc9c0a0f033b","ua":"Mozilla/5.0 (Macintosh) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36"}`
	const ua156 = `{"canvas":"07464365c6ce7358","ua":"Mozilla/5.0 (Macintosh) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/156.0.0.0 Safari/537.36"}`
	s, _ := upgradeFrom(t, 25, func(o *Store) {
		cases := []struct{ id, engine, version, sample string }{
			{"stale", "anybrowser", "0.5.1.156", ua155},
			{"fresh", "anybrowser", "0.5.1.156", ua156},
			{"other", "fury", "0.3.5", ua155},
		}
		for i, c := range cases {
			insertV1Profile(t, o, c.id, "other", "2026-10-01T00:00:00Z")
			if _, err := o.db.Exec(`UPDATE profiles SET engine=?, engine_version=?, fp_baseline=?, fp_last=?,
				fp_verdict='drift', fp_diff='["canvas","ua"]', fp_checked_at='2026-10-02T00:00:00Z', serial=? WHERE id=?`,
				c.engine, c.version, c.sample, c.sample, i+1, c.id); err != nil {
				t.Fatal(err)
			}
		}
	})

	// The 155 sample cannot describe this engine: it is gone, so the next
	// launch measures a fresh baseline instead of drifting forever.
	if b, _ := s.FingerprintBaseline("stale"); b != "" {
		t.Fatalf("stale baseline kept: %s", b)
	}
	if p, err := s.GetProfile("stale"); err != nil || p.FPVerdict != "" || p.FPCheckedAt != "" || len(p.FPDiff) != 0 {
		t.Fatalf("stale verdict = %q %q %v, %v", p.FPVerdict, p.FPCheckedAt, p.FPDiff, err)
	}
	// A baseline of the current build still compares, and another engine
	// that really runs Chromium 155 keeps its own.
	for id, want := range map[string]string{"fresh": ua156, "other": ua155} {
		b, err := s.FingerprintBaseline(id)
		if err != nil || b != want {
			t.Fatalf("%s baseline = %q, %v", id, b, err)
		}
	}
}
