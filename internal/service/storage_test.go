// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile/vm"
	"github.com/D0bby999/anyprofile/internal/store"
)

func TestStorage(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	p, err := s.CreateProfile(ctx, NewProfile{Name: "Stored"})
	if err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(s.Dirs.Profile(p.ID), "Default", "Cache")
	if err := os.MkdirAll(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "f"), make([]byte, 1000), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Dirs.Profile(p.ID), "Default", "Cookies"), make([]byte, 10), 0o600); err != nil {
		t.Fatal(err)
	}
	zz := s.Dirs.Profile("zz")
	if err := os.MkdirAll(zz, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(zz, "a"), make([]byte, 500), 0o600); err != nil {
		t.Fatal(err)
	}

	info, err := s.Storage()
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Orphans) != 1 || info.Orphans[0].ID != "zz" || info.Orphans[0].Bytes != 500 {
		t.Fatalf("orphans = %+v", info.Orphans)
	}
	if info.ProfilesBytes < 1510 {
		t.Fatalf("profilesBytes = %d, want >= 1510", info.ProfilesBytes)
	}

	freed, err := s.ClearCache()
	if err != nil {
		t.Fatal(err)
	}
	if freed != 1000 {
		t.Fatalf("freed = %d, want 1000", freed)
	}
	if _, err := os.Stat(filepath.Join(s.Dirs.Profile(p.ID), "Default", "Cookies")); err != nil {
		t.Fatalf("cookies must survive: %v", err)
	}

	if err := s.DeleteOrphans([]string{"zz", p.ID, "../x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(zz); !os.IsNotExist(err) {
		t.Fatal("orphan zz survived")
	}
	if _, err := os.Stat(s.Dirs.Profile(p.ID)); err != nil {
		t.Fatal("known profile folder was deleted")
	}
}

// DeleteOrphans must only ever remove a folder the scan would list: never the
// profiles root itself, whatever id the caller sends.
func TestDeleteOrphansRefusesRoot(t *testing.T) {
	s, _ := newService(t)
	p, err := s.CreateProfile(context.Background(), NewProfile{Name: "Kept"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(s.Dirs.Profile(p.ID), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Dirs.Profiles(), "loose-file"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteOrphans([]string{"", ".", "..", "loose-file", `a\b`}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{s.Dirs.Profile(p.ID), filepath.Join(s.Dirs.Profiles(), "loose-file")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s was deleted: %v", path, err)
		}
	}
}

func TestDirSizeSparse(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("sparse block accounting is darwin-only")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "userdata.img")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	const virtual int64 = 32 << 30
	if err := f.Truncate(virtual); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if got := dirSize(dir); got >= 1<<20 {
		t.Fatalf("dirSize(sparse 32 GiB) = %d, want < 1 MiB allocated", got)
	}
}

func TestStorageBreakdownAndCleanup(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if _, err := s.CreateProfile(ctx, NewProfile{Name: "Main"}); err != nil {
		t.Fatal(err)
	}

	// Stage APK installer cache + icon (icon must survive ClearCache).
	apkDir := s.apkCacheDir()
	iconsDir := filepath.Join(apkDir, "icons")
	if err := os.MkdirAll(iconsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(apkDir, "com.example.app.apks"), make([]byte, 4096), 0o600); err != nil {
		t.Fatal(err)
	}
	iconPath := filepath.Join(iconsDir, "com.example.app.png")
	if err := os.WriteFile(iconPath, make([]byte, 256), 0o600); err != nil {
		t.Fatal(err)
	}

	// Stage upload pack (must survive ClearCache, deleted by ClearUploads).
	upFile := filepath.Join(s.Dirs.Uploads(), "pack-1", "clip.mp4")
	if err := os.MkdirAll(filepath.Dir(upFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(upFile, make([]byte, 2048), 0o600); err != nil {
		t.Fatal(err)
	}

	// Stage old trace + old unreferenced cookie backup + active cookie backup.
	tracesDir := filepath.Join(s.Dirs.Logs(), "traces")
	cbDir := filepath.Join(s.Dirs.Logs(), "cookie-backups")
	if err := os.MkdirAll(tracesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cbDir, 0o700); err != nil {
		t.Fatal(err)
	}
	oldTrace := filepath.Join(tracesDir, "old.json")
	freshTrace := filepath.Join(tracesDir, "fresh.json")
	if err := os.WriteFile(oldTrace, make([]byte, 1024), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(freshTrace, make([]byte, 512), 0o600); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-30 * 24 * time.Hour)
	_ = os.Chtimes(oldTrace, oldTime, oldTime)

	activeJar := filepath.Join(cbDir, "p1-active.db")
	staleJar := filepath.Join(cbDir, "p1-emptied-old.db")
	_ = os.WriteFile(activeJar, make([]byte, 1024), 0o600)
	_ = os.WriteFile(staleJar, make([]byte, 1024), 0o600)
	_ = os.WriteFile(filepath.Join(cbDir, "p1.json"), []byte(`{"count":5,"backup":"p1-active.db"}`), 0o600)
	_ = os.Chtimes(activeJar, oldTime, oldTime)
	_ = os.Chtimes(staleJar, oldTime, oldTime)

	info, err := s.Storage()
	if err != nil {
		t.Fatal(err)
	}
	if info.APKCacheBytes == 0 || info.UploadsBytes == 0 || info.LogsBytes == 0 || info.TotalBytes < info.APKCacheBytes+info.UploadsBytes {
		t.Fatalf("unexpected storage breakdown: %+v", info)
	}

	freed, err := s.ClearCache()
	if err != nil {
		t.Fatal(err)
	}
	if freed == 0 {
		t.Fatal("ClearCache freed 0 bytes")
	}
	if _, err := os.Stat(filepath.Join(apkDir, "com.example.app.apks")); !os.IsNotExist(err) {
		t.Fatal("cached APK installer survived ClearCache")
	}
	if _, err := os.Stat(iconPath); err != nil {
		t.Fatal("APK icon was removed by ClearCache")
	}
	if _, err := os.Stat(upFile); err != nil {
		t.Fatal("uploads must survive ClearCache")
	}
	if _, err := os.Stat(oldTrace); !os.IsNotExist(err) {
		t.Fatal("stale trace survived ClearCache")
	}
	if _, err := os.Stat(freshTrace); err != nil {
		t.Fatal("fresh trace was removed")
	}
	if _, err := os.Stat(staleJar); !os.IsNotExist(err) {
		t.Fatal("stale unreferenced cookie backup survived")
	}
	if _, err := os.Stat(activeJar); err != nil {
		t.Fatal("active referenced cookie backup was removed")
	}

	upFreed, err := s.ClearUploads()
	if err != nil {
		t.Fatal(err)
	}
	if upFreed == 0 {
		t.Fatal("ClearUploads freed 0 bytes")
	}
	if _, err := os.Stat(upFile); !os.IsNotExist(err) {
		t.Fatal("upload file survived ClearUploads")
	}
	if _, err := os.Stat(s.Dirs.Uploads()); err != nil {
		t.Fatal("uploads root dir must survive ClearUploads")
	}
}

func TestPickMobileIdentityDistinct(t *testing.T) {
	s, _ := newService(t)
	avoDir := filepath.Join(s.Dirs.Mobile(), "avo", "profiles")
	if err := os.MkdirAll(avoDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeProfile := func(id, brand, model string) {
		body := `{"id":"` + id + `","properties":{"ro.product.brand":"` + brand + `","ro.product.manufacturer":"` + brand + `","ro.product.model":"` + model + `","ro.product.device":"` + id + `","ro.product.name":"` + id + `"}}`
		if err := os.WriteFile(filepath.Join(avoDir, id+".json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeProfile("samsung-galaxy-s24-ultra", "samsung", "SM-S928B")
	writeProfile("vivo-x200", "vivo", "V2409")
	writeProfile("nothing-phone-2a", "Nothing", "A142")
	writeProfile("avo-virtual-device", "generic", "AVO")

	// Existing device using legacy alias s23ultra maps to samsung-galaxy-s24-ultra.
	existing := store.MobileDevice{
		ID:           "existing-1",
		Name:         "Phone 1",
		Driver:       "avo",
		Platform:     "darwin",
		RAMMB:        4096,
		CPUs:         3,
		Preset:       "balanced",
		ScreenW:      780,
		ScreenH:      1688,
		Density:      320,
		WithGMS:      true,
		IdentityJSON: vm.MarshalIdentity(vm.IdentityFor("existing-1", "s23ultra")),
	}
	if err := s.Store.CreateMobileDevice(&existing); err != nil {
		t.Fatal(err)
	}

	picked1 := s.pickMobileIdentity("dev-new-1", "", "")
	if picked1.Template == "samsung-galaxy-s24-ultra" || picked1.Template == "avo-virtual-device" {
		t.Fatalf("picked1 should avoid already-used S24 Ultra and virtual device, got %+v", picked1)
	}
	dev2 := existing
	dev2.ID = "existing-2"
	dev2.Name = "Phone 2"
	dev2.IdentityJSON = vm.MarshalIdentity(picked1)
	if err := s.Store.CreateMobileDevice(&dev2); err != nil {
		t.Fatal(err)
	}

	picked2 := s.pickMobileIdentity("dev-new-2", "", "")
	if picked2.Template == "samsung-galaxy-s24-ultra" || picked2.Template == picked1.Template || picked2.Template == "avo-virtual-device" {
		t.Fatalf("picked2 should pick remaining unused model, got %+v (picked1=%s)", picked2, picked1.Template)
	}
}
