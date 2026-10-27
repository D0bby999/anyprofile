// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"

	"github.com/D0bby999/anyprofile/internal/store"
)

func TestBulkFolderMoveRollsBackAndNotifiesAfterSuccess(t *testing.T) {
	s, _ := newService(t)
	p, err := s.CreateProfile(context.Background(), NewProfile{Name: "P"})
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.CreateFolder("F", "")
	if err != nil {
		t.Fatal(err)
	}
	notifications := 0
	s.OnChange = func(kind string) {
		if kind == "profiles" {
			notifications++
		}
	}
	n, err := s.MoveProfilesToFolder([]string{p.ID, "missing"}, f.ID)
	if err == nil || n != 0 {
		t.Fatalf("batch result=%d,%v", n, err)
	}
	got, err := s.Store.GetProfile(p.ID)
	if err != nil || got.FolderID != "" || notifications != 0 {
		t.Fatalf("failed batch mutated state: %+v notifications=%d err=%v", got, notifications, err)
	}
	if n, err := s.MoveProfilesToFolder([]string{p.ID, p.ID}, f.ID); err != nil || n != 1 || notifications != 1 {
		t.Fatalf("success: n=%d notifications=%d err=%v", n, notifications, err)
	}
}

func TestTemplateValidationRejectsUnusableDefaults(t *testing.T) {
	cases := []store.Template{
		{Name: "Engine", Engine: "does-not-exist"},
		{Name: "URL", StartURL: "file:///etc/passwd"},
		{Name: "Folder", FolderID: "missing"},
		{Name: "Languages", Languages: "not_a_language_tag"},
		{Name: "Empty language", Languages: "vi-VN,,en-US"},
		{Name: "Locale", UILocale: "not_a_language_tag"},
		{Name: "Sites", Sites: []string{"missing-site"}},
	}
	for _, tpl := range cases {
		t.Run(tpl.Name, func(t *testing.T) {
			s, _ := newService(t)
			if _, err := s.CreateTemplate(tpl); err == nil {
				t.Fatalf("accepted invalid template: %+v", tpl)
			}
			list, err := s.Store.ListTemplates()
			if err != nil || len(list) != 0 {
				t.Fatalf("invalid template persisted: %+v %v", list, err)
			}
			valid, err := s.CreateTemplate(store.Template{Name: "Valid", Sites: []string{"tiktok"}})
			if err != nil {
				t.Fatal(err)
			}
			tpl.ID = valid.ID
			if err := s.UpdateTemplate(tpl); err == nil {
				t.Fatal("update accepted invalid defaults")
			}
		})
	}
}

func TestTemplateProfileNotificationSeesFinalDefaults(t *testing.T) {
	s, _ := newService(t)
	f, err := s.CreateFolder("F", "")
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := s.CreateTemplate(store.Template{Name: "Custom", Site: "tiktok", Languages: "vi-VN,vi", UILocale: "vi-VN", FolderID: f.ID})
	if err != nil {
		t.Fatal(err)
	}
	notified := 0
	s.OnChange = func(kind string) {
		if kind != "profiles" {
			return
		}
		notified++
		rows, err := s.Store.ListProfiles(store.ProfileFilter{})
		if err != nil {
			t.Error(err)
			return
		}
		if len(rows) != 1 || rows[0].Languages != tpl.Languages || rows[0].FolderID != f.ID {
			t.Errorf("notification preceded final profile write: %+v", rows)
		}
	}
	p, err := s.CreateProfileFromTemplate(context.Background(), tpl.ID, "Profile", "")
	if err != nil || p.FolderID != f.ID || notified != 1 {
		t.Fatalf("result=%+v notifications=%d err=%v", p, notified, err)
	}
}

func TestUpdateBuiltinTemplateRejectsForgedFlag(t *testing.T) {
	s, _ := newService(t)
	if err := s.UpdateTemplate(store.Template{ID: "builtin-tiktok", Name: "Forged", Builtin: false}); err == nil {
		t.Fatal("forged flag bypassed built-in check")
	}
}
