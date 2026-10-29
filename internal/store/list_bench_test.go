// SPDX-License-Identifier: AGPL-3.0-or-later
package store

import (
	"fmt"
	"path/filepath"
	"testing"
)

// Cook Phase 01 measurement harness (temporary, not part of the repo):
// seeds N profiles (one account each) and measures ListProfiles, plus
// prints EXPLAIN QUERY PLAN for the list and search queries.

func seedListBench(t testing.TB, n int) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "bench.db"), filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	for i := 0; i < n; i++ {
		p := &Profile{
			ID:        fmt.Sprintf("p%05d", i),
			Name:      fmt.Sprintf("Kenh %05d", i),
			Engine:    "anybrowser",
			PersonaID: "macos-m4",
			SeedHex:   "0123456789abcdef",
			Platform:  "macos",
			Timezone:  "Asia/Ho_Chi_Minh",
			Languages: "vi-VN,vi",
			UILocale:  "vi-VN",
			Sites:     []string{"tiktok"},
			Tags:      []string{"ads"},
			Notes:     "ghi chu cho kenh",
		}
		if err := s.CreateProfile(p); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.Exec(`INSERT INTO accounts (profile_id, site, login, has_password, has_totp, secret_key) VALUES (?, 'tiktok', ?, 1, 0, ?)`,
			p.ID, fmt.Sprintf("kenh%05d", i), p.ID); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func BenchmarkListProfiles(b *testing.B) {
	for _, n := range []int{100, 300, 500} {
		s := seedListBench(b, n)
		b.Run(fmt.Sprintf("all-%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				out, err := s.ListProfiles(ProfileFilter{})
				if err != nil || len(out) != n {
					b.Fatalf("err=%v len=%d want %d", err, len(out), n)
				}
			}
		})
		b.Run(fmt.Sprintf("search-%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				out, err := s.ListProfiles(ProfileFilter{Query: "kenh"})
				if err != nil || len(out) != n {
					b.Fatalf("err=%v len=%d want %d", err, len(out), n)
				}
			}
		})
		b.Run(fmt.Sprintf("page50-%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				out, err := s.ListProfiles(ProfileFilter{Limit: 50})
				if err != nil || len(out) != min(50, n) {
					b.Fatalf("err=%v len=%d", err, len(out))
				}
			}
		})
		b.Run(fmt.Sprintf("tag-%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				out, err := s.ListProfiles(ProfileFilter{Tag: "ads"})
				if err != nil || len(out) != n {
					b.Fatalf("err=%v len=%d want %d", err, len(out), n)
				}
			}
		})
	}
}

func TestListExplain(t *testing.T) {
	s := seedListBench(t, 300)
	queries := map[string]string{
		"list":         `SELECT ` + profileCols + ` FROM profiles WHERE deleted_at IS NULL ORDER BY pinned DESC, sort_order, created_at`,
		"search":       `SELECT ` + profileCols + ` FROM profiles WHERE deleted_at IS NULL AND (name LIKE '%kenh%' ESCAPE '\' OR EXISTS (SELECT 1 FROM accounts a WHERE a.profile_id = profiles.id AND a.login LIKE '%kenh%' ESCAPE '\') OR notes LIKE '%kenh%' ESCAPE '\' OR tags LIKE '%kenh%' ESCAPE '\') ORDER BY pinned DESC, sort_order, created_at`,
		"accounts-all": `SELECT profile_id, ` + accountCols + ` FROM accounts ORDER BY rowid`,
	}
	for name, q := range queries {
		rows, err := s.db.Query(`EXPLAIN QUERY PLAN ` + q)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		t.Logf("== %s ==", name)
		for rows.Next() {
			var id, parent, notused int
			var detail string
			if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
				t.Fatal(err)
			}
			t.Logf("  %s", detail)
		}
		rows.Close()
	}
}
