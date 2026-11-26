// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package avo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/D0bby999/anyprofile/internal/mobile"
)

func TestCloneDeviceClonesDisksAndMarksReinit(t *testing.T) {
	avoRoot := t.TempDir()
	for _, rel := range []string{
		"bin/anyprofile-vmm",
		"bin/anyprofile-view",
		"bin/adb",
		"gfxstream/lib64/libgfxstream_backend.dylib",
		"image/v1/boot.img",
		"image/v1/vendor_boot.img",
		"image/v1/super.img",
		"image/v1/userdata.img",
	} {
		p := filepath.Join(avoRoot, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(avoRoot, "image/current"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	devRoot := t.TempDir()
	d := New(filepath.Join(avoRoot, "qemu"), devRoot, nil)
	d.avoRoot = avoRoot

	src := mobile.Device{ID: "master1", Name: "Master"}
	dst := mobile.Device{ID: "clone1", Name: "Clone #1"}
	srcInst := d.instDir(src)
	if err := os.MkdirAll(srcInst, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, f := range instanceDisks {
		if err := os.WriteFile(filepath.Join(srcInst, f), []byte("disk-"+f), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	_ = os.WriteFile(filepath.Join(srcInst, ".gpu_layers_ready"), []byte("1\n"), 0o600)

	ref, imgVer, err := d.CloneDevice(context.Background(), src, dst, true)
	if err != nil {
		t.Fatalf("CloneDevice: %v", err)
	}
	if ref != dst.ID || imgVer != "cuttlefish-v1" {
		t.Fatalf("got ref=%q imgVer=%q", ref, imgVer)
	}
	dstInst := d.instDir(dst)
	for _, f := range instanceDisks {
		got, err := os.ReadFile(filepath.Join(dstInst, f))
		if err != nil || string(got) != "disk-"+f {
			t.Fatalf("disk %s: got %q err %v", f, got, err)
		}
	}
	mark, err := os.ReadFile(filepath.Join(dstInst, cloneReinitMarker))
	if err != nil || strings.TrimSpace(string(mark)) != "full" {
		t.Fatalf("expected full reinit marker, got %q err %v", mark, err)
	}
	if !exists(filepath.Join(dstInst, ".gpu_layers_ready")) {
		t.Fatalf("expected .gpu_layers_ready copied")
	}
}

func TestPunchZeroHolesPreservesData(t *testing.T) {
	p := filepath.Join(t.TempDir(), "userdata.img")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	// Write 1 MiB non-zero, 4 MiB zeros, 1 MiB non-zero.
	data := make([]byte, 1<<20)
	for i := range data {
		data[i] = 0x5A
	}
	zeros := make([]byte, 4<<20)
	if _, err := f.Write(data); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(zeros); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := punchZeroHoles(p); err != nil {
		t.Fatalf("punchZeroHoles: %v", err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 6<<20 {
		t.Fatalf("logical size changed: %d", len(raw))
	}
	if raw[0] != 0x5A || raw[(1<<20)-1] != 0x5A || raw[1<<20] != 0 || raw[(5<<20)] != 0x5A {
		t.Fatalf("data corrupted after punchZeroHoles")
	}
}
