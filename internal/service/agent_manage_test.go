// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"
	"testing"
)

// The agent's patch changes only what it names: a nil field keeps its value.
func TestPatchProfileLeavesUnsetFields(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	p, err := s.CreateProfile(ctx, NewProfile{Name: "Kênh", Sites: []string{"tiktok"}, Tags: []string{"vn"}})
	if err != nil {
		t.Fatal(err)
	}
	notes := "  giữ chỗ  "
	got, err := s.PatchProfile(ctx, p.ID, ProfilePatch{Notes: &notes})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Kênh" || len(got.Sites) != 1 || got.Sites[0] != "tiktok" || len(got.Tags) != 1 || got.Tags[0] != "vn" {
		t.Fatalf("patch reset a field it did not name: %+v", got)
	}
	if got.Notes != "  giữ chỗ  " {
		t.Fatalf("notes = %q", got.Notes)
	}
}

func TestPatchProfileRenamesAndCleansTags(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	p, err := s.CreateProfile(ctx, NewProfile{Name: "Cũ"})
	if err != nil {
		t.Fatal(err)
	}
	name := "  Mới  "
	tags := []string{" vn ", "vn", "", "  ", "shop"}
	got, err := s.PatchProfile(ctx, p.ID, ProfilePatch{Name: &name, Tags: &tags})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Mới" {
		t.Fatalf("name = %q", got.Name)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "vn" || got.Tags[1] != "shop" {
		t.Fatalf("tags = %v, want [vn shop]", got.Tags)
	}

	empty := "   "
	if _, err := s.PatchProfile(ctx, p.ID, ProfilePatch{Name: &empty}); err == nil ||
		!strings.Contains(err.Error(), "name_required") {
		t.Fatalf("empty name err = %v", err)
	}
}

// A browser keeps the network it started with, so a proxy change is refused
// while the profile is open instead of silently making the row a lie.
func TestPatchProfileRefusesAProxyChangeWhileOpen(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if _, err := s.ImportProxies(ctx, "gw.example:10001:u:pw", ""); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.ListProxies()
	p, err := s.CreateProfile(ctx, NewProfile{Name: "Đang mở", ProxyID: rows[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	s.Running = func(id string) bool { return id == p.ID }
	if _, err := s.AssignProxy(ctx, p.ID, ""); err == nil || !strings.Contains(err.Error(), "profile_running") {
		t.Fatalf("proxy change on an open profile err = %v", err)
	}
	// Another field is still patchable: only the network is frozen.
	notes := "ok"
	if _, err := s.PatchProfile(ctx, p.ID, ProfilePatch{Notes: &notes}); err != nil {
		t.Fatalf("notes on an open profile: %v", err)
	}
}

// Assigning another proxy re-derives the network of the new exit.
func TestAssignProxyRederivesNetwork(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if _, err := s.ImportProxies(ctx, "gw.example:10001:u:pw\ngw.example:10002:u:pw", ""); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.ListProxies()
	var vn string
	for _, r := range rows {
		if r.Port == 10002 {
			vn = r.ID
		}
	}
	p, err := s.CreateProfile(ctx, NewProfile{Name: "Đổi proxy", ProxyID: rows[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.AssignProxy(ctx, p.ID, vn)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProxyID != vn || got.Timezone != "Asia/Ho_Chi_Minh" || got.UILocale != "vi" {
		t.Fatalf("after assign = %+v", got)
	}
	if _, err := s.AssignProxy(ctx, p.ID, "nope"); err == nil || !strings.Contains(err.Error(), "proxy_not_found") {
		t.Fatalf("unknown proxy err = %v", err)
	}
}

// Agents may change profiles only after the user turns it on; the flag
// survives a port change because Settings sends the whole struct.
func TestAgentManageSetting(t *testing.T) {
	s, _ := newService(t)
	if s.APISettings().AgentManage {
		t.Fatal("agent management must start off")
	}
	if err := s.SetAPISettings(APISettings{Enabled: true, Port: 51525, AgentManage: true}); err != nil {
		t.Fatal(err)
	}
	if got := s.APISettings(); !got.AgentManage || !got.Enabled {
		t.Fatalf("after set = %+v", got)
	}
	if err := s.SetAPISettings(APISettings{Enabled: true, Port: 51526, AgentManage: true}); err != nil {
		t.Fatal(err)
	}
	if got := s.APISettings(); !got.AgentManage || got.Port != 51526 {
		t.Fatalf("port change lost the flag: %+v", got)
	}
	if err := s.SetAPISettings(APISettings{Enabled: true, Port: 51526}); err != nil {
		t.Fatal(err)
	}
	if s.APISettings().AgentManage {
		t.Fatal("turning it off did not persist")
	}
}
