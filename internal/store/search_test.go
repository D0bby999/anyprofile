// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"fmt"
	"strings"
	"testing"
)

func TestProfileSearchFoldsVietnameseAndLiteralWildcards(t *testing.T) {
	s := seedFolderTest(t)
	cases := []struct {
		id, name, notes, query string
		tags                   []string
	}{
		{id: "name", name: "Kênh KHÁCH ĐẶC BIỆT", query: "kenh khach dac biet"},
		{id: "notes", name: "Other", notes: "Hà Nội", query: "ha noi"},
		{id: "tags", name: "Tags", tags: []string{"Đối tác"}, query: "doi tac"},
		{id: "literal", name: "100%_done", query: "100%_"},
	}
	for _, c := range cases {
		mkProfile(t, s, c.id)
		p, err := s.GetProfile(c.id)
		if err != nil {
			t.Fatal(err)
		}
		p.Name, p.Notes, p.Tags = c.name, c.notes, c.tags
		if err := s.UpdateProfile(p); err != nil {
			t.Fatal(err)
		}
	}
	mkProfile(t, s, "decoy")
	p, _ := s.GetProfile("decoy")
	p.Name = "100anything_done"
	if err := s.UpdateProfile(p); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		rows, err := s.ListProfiles(ProfileFilter{Query: c.query, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		n, err := s.CountProfiles(ProfileFilter{Query: c.query})
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 || len(rows) != 1 || rows[0].ID != c.id {
			t.Fatalf("query %q: rows=%+v total=%d", c.query, rows, n)
		}
	}
	if err := s.SaveAccount("name", Account{Site: "tiktok", Login: "ĐĂNG NHẬP", SecretKey: "search-test"}); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListProfiles(ProfileFilter{Query: "dang nhap"})
	if err != nil || len(rows) != 1 {
		t.Fatalf("account search=%+v err=%v", rows, err)
	}
}

func TestPaging500ProfilesWithTiedTimestamps(t *testing.T) {
	s := seedPageTest(t, 500)
	if _, err := s.db.Exec(`UPDATE profiles SET created_at='2026-01-01T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for offset := 0; offset < 500; offset += 50 {
		rows, err := s.ListProfiles(ProfileFilter{Limit: 50, Offset: offset})
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 50 {
			t.Fatalf("page offset %d length=%d", offset, len(rows))
		}
		for i, p := range rows {
			want := fmt.Sprintf("p%03d", offset+i)
			if seen[p.ID] || p.ID != want {
				t.Fatalf("unstable page: got %q want %q", p.ID, want)
			}
			seen[p.ID] = true
		}
	}
	rows, err := s.ListProfiles(ProfileFilter{Query: "KENH 4", Limit: 50})
	if err != nil || len(rows) != 50 {
		t.Fatalf("search page len=%d err=%v", len(rows), err)
	}
	n, err := s.CountProfiles(ProfileFilter{Query: "kenh 4"})
	if err != nil || n != 100 {
		t.Fatalf("search count=%d err=%v", n, err)
	}
}

func TestProfileListUsesOrderIndex(t *testing.T) {
	s := seedFolderTest(t)
	rows, err := s.db.Query(`EXPLAIN QUERY PLAN SELECT id FROM profiles WHERE deleted_at IS NULL ORDER BY pinned DESC,sort_order,created_at,id LIMIT 50`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	text := strings.Join(plan, "; ")
	t.Log(text)
	if !strings.Contains(text, "profiles_list_order") || strings.Contains(text, "TEMP B-TREE") {
		t.Fatalf("unexpected plan: %s", text)
	}
}
