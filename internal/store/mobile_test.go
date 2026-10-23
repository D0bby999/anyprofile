// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestMobileMigrationFrom15(t *testing.T) {
	s, dir := upgradeFrom(t, 15, func(o *Store) {
		insertV1Profile(t, o, "p1", "other", "2026-10-01T00:00:00Z")
	})
	if p, err := s.GetProfile("p1"); err != nil || p.ID != "p1" {
		t.Fatalf("profile after upgrade = %+v, %v", p, err)
	}
	if b, _ := filepath.Glob(filepath.Join(dir, "anyprofile-v15-*.db")); len(b) != 1 {
		t.Fatalf("backup files = %v", b)
	}
	if ds, err := s.ListMobileDevices(ProfileFilter{}); err != nil || len(ds) != 0 {
		t.Fatalf("devices = %v, %v", ds, err)
	}
}

func TestMobileDevices(t *testing.T) {
	s, _ := openTemp(t)
	px := Proxy{ID: "px", Label: "p", Scheme: "socks5", Host: "h", Port: 1}
	if err := s.CreateProxy(&px); err != nil {
		t.Fatal(err)
	}
	a := MobileDevice{ID: "a", Name: "Phone A", Driver: "vm", Platform: "darwin", ProxyID: "px",
		RAMMB: 3072, CPUs: 4, Tags: []string{"vn"}}
	b := MobileDevice{ID: "b", Name: "Phone B", Driver: "vm", Platform: "darwin", RAMMB: 2048, CPUs: 2}
	for _, d := range []*MobileDevice{&a, &b} {
		if err := s.CreateMobileDevice(d); err != nil {
			t.Fatal(err)
		}
	}
	if a.Serial != 1 || b.Serial != 2 {
		t.Fatalf("serials %d %d", a.Serial, b.Serial)
	}
	if err := s.SetMobileDriverState("a", "dir-a", "img-1"); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetMobileDevice("a")
	if err != nil || got.DriverRef != "dir-a" || got.ImageVersion != "img-1" || got.ProxyID != "px" || got.Tags[0] != "vn" {
		t.Fatalf("get = %+v, %v", got, err)
	}
	got.Name, got.Pinned, got.ProxyID = "Renamed", true, ""
	if err := s.UpdateMobileDevice(got); err != nil {
		t.Fatal(err)
	}
	for q, want := range map[string]int{"": 2, "renamed": 1, "#2": 1, "vn": 1, "nothing": 0} {
		ds, err := s.ListMobileDevices(ProfileFilter{Query: q})
		if err != nil || len(ds) != want {
			t.Errorf("query %q: %d devices, want %d (%v)", q, len(ds), want, err)
		}
	}
	if ds, _ := s.ListMobileDevices(ProfileFilter{}); ds[0].ID != "a" {
		t.Fatal("the pinned device is not first")
	}
	if ds, _ := s.ListMobileDevices(ProfileFilter{Tag: "vn"}); len(ds) != 1 {
		t.Fatal("tag filter")
	}

	if err := s.TrashMobileDevice("b"); err != nil {
		t.Fatal(err)
	}
	if ds, _ := s.ListMobileDevices(ProfileFilter{Trashed: true}); len(ds) != 1 || ds[0].ID != "b" {
		t.Fatalf("trash = %+v", ds)
	}
	if err := s.RestoreMobileDevice("b"); err != nil {
		t.Fatal(err)
	}

	if err := s.PutMobileLaunch(MobileLaunch{DeviceID: "a", PID: 7, PIDStartTime: "t", ADBPort: 5555}); err != nil {
		t.Fatal(err)
	}
	ls, err := s.ListMobileLaunches()
	if err != nil || len(ls) != 1 || ls[0].PID != 7 || ls[0].ADBPort != 5555 {
		t.Fatalf("launches = %+v, %v", ls, err)
	}
	if err := s.PurgeMobileDevice("a"); err != nil {
		t.Fatal(err)
	}
	if ls, _ := s.ListMobileLaunches(); len(ls) != 0 {
		t.Fatal("launch outlived its device")
	}
	if _, err := s.GetMobileDevice("a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("purged device: %v", err)
	}
	if err := s.MarkMobileOpened("a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("mark a missing device: %v", err)
	}
}
