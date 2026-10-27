// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"path/filepath"
	"testing"
)

func TestTemplatesCRUD(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "tpl.db"), filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	tpl := &Template{ID: "t1", Name: "Kênh bán hàng", Site: "facebook",
		Sites: []string{"facebook", "instagram"}, ProxyNote: "Proxy dân cư VN",
		Languages: "vi-VN,vi", UILocale: "vi-VN", Tags: []string{"shop"}}
	if err := s.CreateTemplate(tpl); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTemplate("t1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Site != "facebook" || len(got.Sites) != 2 || got.Sites[1] != "instagram" ||
		got.ProxyNote != "Proxy dân cư VN" || len(got.Tags) != 1 {
		t.Fatalf("roundtrip = %+v", got)
	}
	got.Name = "Kênh livestream"
	if err := s.UpdateTemplate(&got); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListTemplates()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "Kênh livestream" {
		t.Fatalf("list = %+v", list)
	}
	if err := s.DeleteTemplate("t1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetTemplate("t1"); err != ErrNotFound {
		t.Fatalf("after delete err = %v, want ErrNotFound", err)
	}
}
