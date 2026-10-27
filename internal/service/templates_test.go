// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCreateProfileFromBuiltinTemplate(t *testing.T) {
	s, _ := newService(t)
	p, err := s.CreateProfileFromTemplate(context.Background(), "builtin-tiktok", "Kênh A", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Sites) != 1 || p.Sites[0] != "tiktok" {
		t.Fatalf("sites = %v, want [tiktok]", p.Sites)
	}
	if p.Languages != "vi-VN,vi" || p.UILocale != "vi-VN" {
		t.Fatalf("languages = %q locale = %q, want the template's vi-VN", p.Languages, p.UILocale)
	}
	if p.StartURL != "https://www.tiktok.com/" {
		t.Fatalf("start URL = %q", p.StartURL)
	}
}

func TestBuiltinTemplatesJSONHasNoSecretFields(t *testing.T) {
	s, _ := newService(t)
	list, err := s.ListTemplates()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 4 {
		t.Fatalf("templates = %d, want at least the 4 built-ins", len(list))
	}
	for _, tpl := range list {
		b, err := json.Marshal(tpl)
		if err != nil {
			t.Fatal(err)
		}
		low := strings.ToLower(string(b))
		for _, bad := range []string{"password", "secret", "cookie", "token", "proxyid"} {
			if strings.Contains(low, bad) {
				t.Fatalf("template %s JSON contains %q: %s", tpl.ID, bad, b)
			}
		}
	}
}

func TestBuiltinTemplateIsReadOnlyButDuplicable(t *testing.T) {
	s, _ := newService(t)
	if err := s.DeleteTemplate("builtin-tiktok"); err == nil {
		t.Fatal("deleting a built-in template succeeded")
	}
	cp, err := s.DuplicateTemplate("builtin-tiktok")
	if err != nil {
		t.Fatal(err)
	}
	if cp.Builtin || cp.ID == "builtin-tiktok" || !strings.Contains(cp.Name, "TikTok") {
		t.Fatalf("duplicate = %+v", cp)
	}
	if err := s.DeleteTemplate(cp.ID); err != nil {
		t.Fatalf("deleting the user copy: %v", err)
	}
}
