// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"fmt"
	"path/filepath"
	"testing"
)

func seedPageTest(t *testing.T, n int) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "page.db"), filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if _, err := s.db.Exec(`INSERT INTO extensions (id, name, version, created_at) VALUES ('ext-a', 'Ext A', '1.0', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		p := &Profile{
			ID:        fmt.Sprintf("p%03d", i),
			Name:      fmt.Sprintf("Kenh %03d", i),
			Engine:    "apostate",
			PersonaID: "macos-m4",
			SeedHex:   "0123456789abcdef",
			Platform:  "macos",
			Timezone:  "Asia/Ho_Chi_Minh",
			Languages: "vi-VN,vi",
			UILocale:  "vi-VN",
			Sites:     []string{"tiktok"},
			Tags:      []string{"ads"},
		}
		if err := s.CreateProfile(p); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.Exec(`INSERT INTO accounts (profile_id, site, login, has_password, has_totp, secret_key) VALUES (?, 'tiktok', ?, 1, 0, ?)`,
			p.ID, "kenh"+p.ID, p.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.Exec(`INSERT INTO profile_extensions (profile_id, extension_id) VALUES (?, 'ext-a')`, p.ID); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestListProfilesPagingIsDisjointAndComplete(t *testing.T) {
	s := seedPageTest(t, 25)
	seen := map[string]bool{}
	for offset := 0; offset < 25; offset += 10 {
		page, err := s.ListProfiles(ProfileFilter{Limit: 10, Offset: offset})
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range page {
			if seen[p.ID] {
				t.Fatalf("profile %s appears on two pages", p.ID)
			}
			seen[p.ID] = true
			if len(p.Accounts) != 1 || p.Accounts[0].Login != "kenh"+p.ID {
				t.Fatalf("page row %s has accounts %+v, want its own one account", p.ID, p.Accounts)
			}
			if len(p.Extensions) != 1 || p.Extensions[0] != "ext-a" {
				t.Fatalf("page row %s has extensions %v, want [ext-a]", p.ID, p.Extensions)
			}
		}
	}
	if len(seen) != 25 {
		t.Fatalf("pages covered %d profiles, want 25", len(seen))
	}
	total, err := s.CountProfiles(ProfileFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 25 {
		t.Fatalf("count = %d, want 25", total)
	}
}

func TestListProfilesSearchSubsetAndCount(t *testing.T) {
	s := seedPageTest(t, 30)
	got, err := s.ListProfiles(ProfileFilter{Query: "kenh 01"})
	if err != nil {
		t.Fatal(err)
	}
	// "kenh 01" matches Kenh 01 plus Kenh 010..019 style names: with the
	// zero-padded names here that is exactly Kenh 010..Kenh 019 minus
	// none — compute from the seeded names instead of guessing.
	want := 0
	for i := 0; i < 30; i++ {
		name := fmt.Sprintf("Kenh %03d", i)
		if containsFold(name, "kenh 01") {
			want++
		}
	}
	if len(got) != want {
		t.Fatalf("search returned %d, want %d", len(got), want)
	}
	total, err := s.CountProfiles(ProfileFilter{Query: "kenh 01"})
	if err != nil {
		t.Fatal(err)
	}
	if total != want {
		t.Fatalf("search count = %d, want %d", total, want)
	}
}

func containsFold(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			ok := true
			for j := 0; j < len(sub); j++ {
				a, b := s[i+j], sub[j]
				if 'A' <= a && a <= 'Z' {
					a += 'a' - 'A'
				}
				if 'A' <= b && b <= 'Z' {
					b += 'a' - 'A'
				}
				if a != b {
					ok = false
					break
				}
			}
			if ok {
				return true
			}
		}
		return false
	})()
}
