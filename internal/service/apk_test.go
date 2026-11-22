// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/store"
)

func TestAPKProGateAndInspect(t *testing.T) {
	s, _ := newMobileService(t, "") // Pro OFF
	ctx := context.Background()

	apkFile := writeMiniAPK(t, t.TempDir(), "com.example.demo", "Demo App", "lib/arm64-v8a/libdemo.so")
	if _, err := s.InspectAPK(ctx, apkFile); err == nil {
		t.Fatal("InspectAPK must be blocked without Pro")
	} else {
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Code != "pro_required" {
			t.Fatalf("expected pro_required, got %v", err)
		}
	}
	if _, err := s.AddAPK(ctx, APKInput{FilePath: apkFile}); err == nil {
		t.Fatal("AddAPK must be blocked without Pro")
	}
	if err := s.LaunchAPK(ctx, "com.example.demo", ""); err == nil {
		t.Fatal("LaunchAPK must be blocked without Pro")
	}

	// Lapsed Pro can still list and delete existing records.
	rec := store.MobileAPK{Package: "com.example.old", Label: "Old App"}
	if err := s.Store.UpsertMobileAPK(&rec); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListAPKs()
	if err != nil || len(list) != 1 {
		t.Fatalf("ListAPKs without Pro = %v, %v", list, err)
	}
	if err := s.DeleteAPK(ctx, "com.example.old", false); err != nil {
		t.Fatalf("DeleteAPK without Pro: %v", err)
	}

	// With Pro enabled: InspectAPK parses offline and checks ABI.
	sPro, _ := newMobileService(t, "active")
	info, err := sPro.InspectAPK(ctx, apkFile)
	if err != nil {
		t.Fatalf("InspectAPK with Pro: %v", err)
	}
	if info.Package != "com.example.demo" || info.Label != "Demo App" || !info.ABICompatible {
		t.Fatalf("unexpected inspect result: %+v", info)
	}
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		badFile := writeMiniAPK(t, t.TempDir(), "com.example.x86", "OldX86", "lib/x86_64/libold.so")
		badInfo, err := sPro.InspectAPK(ctx, badFile)
		if err != nil {
			t.Fatalf("InspectAPK bad ABI: %v", err)
		}
		if badInfo.ABICompatible {
			t.Fatal("expected x86_64-only APK flagged incompatible on darwin/arm64")
		}
	}

	// Dedicated APK runner host is created separately from user Mobile profiles
	// and stays hidden from ListMobileDevices.
	if _, err := sPro.CreateMobileDevice(ctx, MobileInput{Name: "User Phone"}); err != nil {
		t.Fatalf("CreateMobileDevice: %v", err)
	}
	hostID, err := sPro.ensureAPKDevice(ctx, "")
	if err != nil || hostID == "" {
		t.Fatalf("ensureAPKDevice: %q, %v", hostID, err)
	}
	hostID2, err := sPro.ensureAPKDevice(ctx, "")
	if err != nil || hostID2 != hostID {
		t.Fatalf("ensureAPKDevice not idempotent: %q vs %q (%v)", hostID, hostID2, err)
	}
	rows, err := sPro.ListMobileDevices(store.ProfileFilter{})
	if err != nil {
		t.Fatalf("ListMobileDevices: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "User Phone" || rows[0].ID == hostID {
		t.Fatalf("hidden APK runner leaked into Mobile list: %+v", rows)
	}

	// UpdateAPK validates and persists custom name, proxy, orientation, and notes.
	apkRec := store.MobileAPK{Package: "com.example.demo", Label: "Demo App", Orientation: "auto"}
	if err := sPro.Store.UpsertMobileAPK(&apkRec); err != nil {
		t.Fatal(err)
	}
	p := store.Proxy{ID: "px-us", Label: "US Exit", Scheme: "http", Host: "1.2.3.4", Port: 8080}
	if err := sPro.Store.CreateProxy(&p); err != nil {
		t.Fatal(err)
	}
	updated, err := sPro.UpdateAPK(ctx, APKUpdate{
		Package:          "com.example.demo",
		Label:            "Demo US",
		Orientation:      "landscape",
		ProxyID:          p.ID,
		Notes:            "Shop account",
		IdentityTemplate: "pixel8",
	})
	if err != nil {
		t.Fatalf("UpdateAPK: %v", err)
	}
	if updated.Label != "Demo US" || updated.ProxyID != p.ID || updated.Orientation != "landscape" || updated.Notes != "Shop account" {
		t.Fatalf("unexpected updated APK: %+v", updated)
	}
	if updated.IdentityJSON == "" || updated.AndroidID == "" {
		t.Fatalf("expected hardware identity and androidId, got %+v", updated)
	}
	// Rotating identity generates a new IMEI/Serial and Android ID.
	rotated, err := sPro.UpdateAPK(ctx, APKUpdate{
		Package:          "com.example.demo",
		Label:            "Demo US",
		Orientation:      "landscape",
		ProxyID:          p.ID,
		Notes:            "Shop account",
		IdentityTemplate: "regen:pixel8",
	})
	if err != nil {
		t.Fatalf("UpdateAPK regen: %v", err)
	}
	if rotated.IdentityJSON == updated.IdentityJSON || rotated.AndroidID == updated.AndroidID {
		t.Fatalf("expected regen to rotate identity and androidId: before=%+v after=%+v", updated, rotated)
	}
}

func writeMiniAPK(t *testing.T, dir, pkg, label, libEntry string) string {
	t.Helper()
	p := filepath.Join(dir, pkg+".apk")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("AndroidManifest.xml")
	_, _ = w.Write(miniManifestAXML(pkg, label))
	if libEntry != "" {
		lw, _ := zw.Create(libEntry)
		_, _ = lw.Write([]byte("elf"))
	}
	_ = zw.Close()
	_ = f.Close()
	return p
}

func miniManifestAXML(pkg, label string) []byte {
	const ns = "http://schemas.android.com/apk/res/android"
	pool := []string{ns, "manifest", "package", pkg, "application", "label", label}
	var spData bytes.Buffer
	offs := make([]uint32, len(pool))
	for i, s := range pool {
		offs[i] = uint32(spData.Len())
		spData.WriteByte(byte(len(s)))
		spData.WriteByte(byte(len(s)))
		spData.WriteString(s)
		spData.WriteByte(0)
	}
	for spData.Len()%4 != 0 {
		spData.WriteByte(0)
	}
	var sp bytes.Buffer
	_ = binary.Write(&sp, binary.LittleEndian, uint16(0x0001))
	_ = binary.Write(&sp, binary.LittleEndian, uint16(28))
	_ = binary.Write(&sp, binary.LittleEndian, uint32(28+len(pool)*4+spData.Len()))
	_ = binary.Write(&sp, binary.LittleEndian, uint32(len(pool)))
	_ = binary.Write(&sp, binary.LittleEndian, uint32(0))
	_ = binary.Write(&sp, binary.LittleEndian, uint32(0x100))
	_ = binary.Write(&sp, binary.LittleEndian, uint32(28+len(pool)*4))
	_ = binary.Write(&sp, binary.LittleEndian, uint32(0))
	for _, o := range offs {
		_ = binary.Write(&sp, binary.LittleEndian, o)
	}
	sp.Write(spData.Bytes())

	var body bytes.Buffer
	startElem := func(nameIdx uint32, attrNS, attrName, attrVal uint32) {
		_ = binary.Write(&body, binary.LittleEndian, uint16(0x0102))
		_ = binary.Write(&body, binary.LittleEndian, uint16(16))
		_ = binary.Write(&body, binary.LittleEndian, uint32(56))
		_ = binary.Write(&body, binary.LittleEndian, uint32(1))
		_ = binary.Write(&body, binary.LittleEndian, uint32(0xffffffff))
		_ = binary.Write(&body, binary.LittleEndian, uint32(0xffffffff))
		_ = binary.Write(&body, binary.LittleEndian, nameIdx)
		_ = binary.Write(&body, binary.LittleEndian, uint16(20))
		_ = binary.Write(&body, binary.LittleEndian, uint16(20))
		_ = binary.Write(&body, binary.LittleEndian, uint16(1))
		_ = binary.Write(&body, binary.LittleEndian, uint16(0))
		_ = binary.Write(&body, binary.LittleEndian, uint16(0))
		_ = binary.Write(&body, binary.LittleEndian, uint16(0))
		_ = binary.Write(&body, binary.LittleEndian, attrNS)
		_ = binary.Write(&body, binary.LittleEndian, attrName)
		_ = binary.Write(&body, binary.LittleEndian, attrVal)
		_ = binary.Write(&body, binary.LittleEndian, uint16(8))
		body.WriteByte(0)
		body.WriteByte(0x03)
		_ = binary.Write(&body, binary.LittleEndian, attrVal)
	}
	endElem := func(nameIdx uint32) {
		_ = binary.Write(&body, binary.LittleEndian, uint16(0x0103))
		_ = binary.Write(&body, binary.LittleEndian, uint16(16))
		_ = binary.Write(&body, binary.LittleEndian, uint32(24))
		_ = binary.Write(&body, binary.LittleEndian, uint32(1))
		_ = binary.Write(&body, binary.LittleEndian, uint32(0xffffffff))
		_ = binary.Write(&body, binary.LittleEndian, uint32(0xffffffff))
		_ = binary.Write(&body, binary.LittleEndian, nameIdx)
	}
	startElem(1, 0xffffffff, 2, 3)
	startElem(4, 0, 5, 6)
	endElem(4)
	endElem(1)

	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, uint16(0x0003))
	_ = binary.Write(&out, binary.LittleEndian, uint16(8))
	_ = binary.Write(&out, binary.LittleEndian, uint32(8+sp.Len()+body.Len()))
	out.Write(sp.Bytes())
	out.Write(body.Bytes())
	return out.Bytes()
}
