// SPDX-License-Identifier: MIT

package apk

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// buildTestAXML constructs a minimal valid Android binary XML document for unit tests.
func buildTestAXML(pkg, label, activity string, orientation int32) []byte {
	pool := []string{
		androidNS,
		"manifest",
		"package",
		pkg,
		"versionName",
		"2.4.1",
		"versionCode",
		"uses-sdk",
		"minSdkVersion",
		"targetSdkVersion",
		"application",
		"label",
		label,
		"activity",
		"name",
		activity,
		"screenOrientation",
		"intent-filter",
		"action",
		"android.intent.action.MAIN",
		"category",
		"android.intent.category.LAUNCHER",
	}
	idx := func(s string) uint32 {
		for i, v := range pool {
			if v == s {
				return uint32(i)
			}
		}
		panic("missing string: " + s)
	}

	sp := buildUTF8StringPool(pool)

	var body bytes.Buffer
	writeStartElem := func(name string, attrs [][5]uint32) {
		chunkSize := uint32(36 + len(attrs)*20)
		writeU16(&body, resXMLStartElement)
		writeU16(&body, 16)
		writeU32(&body, chunkSize)
		writeU32(&body, 1) // line
		writeU32(&body, 0xffffffff)
		writeU32(&body, 0xffffffff) // ns
		writeU32(&body, idx(name))
		writeU16(&body, 20) // attrStart
		writeU16(&body, 20) // attrSize
		writeU16(&body, uint16(len(attrs)))
		writeU16(&body, 0)
		writeU16(&body, 0)
		writeU16(&body, 0)
		for _, a := range attrs {
			writeU32(&body, a[0]) // ns
			writeU32(&body, a[1]) // name
			writeU32(&body, a[2]) // raw
			writeU16(&body, 8)    // typed size
			body.WriteByte(0)
			body.WriteByte(byte(a[3])) // dataType
			writeU32(&body, a[4])      // data
		}
	}
	writeEndElem := func(name string) {
		writeU16(&body, resXMLEndElement)
		writeU16(&body, 16)
		writeU32(&body, 24)
		writeU32(&body, 1)
		writeU32(&body, 0xffffffff)
		writeU32(&body, 0xffffffff)
		writeU32(&body, idx(name))
	}

	nsA := idx(androidNS)
	writeStartElem("manifest", [][5]uint32{
		{0xffffffff, idx("package"), idx(pkg), typeString, idx(pkg)},
		{nsA, idx("versionName"), idx("2.4.1"), typeString, idx("2.4.1")},
		{nsA, idx("versionCode"), 0xffffffff, typeIntDec, 42},
	})
	writeStartElem("uses-sdk", [][5]uint32{
		{nsA, idx("minSdkVersion"), 0xffffffff, typeIntDec, 24},
		{nsA, idx("targetSdkVersion"), 0xffffffff, typeIntDec, 35},
	})
	writeEndElem("uses-sdk")
	writeStartElem("application", [][5]uint32{
		{nsA, idx("label"), idx(label), typeString, idx(label)},
	})
	writeStartElem("activity", [][5]uint32{
		{nsA, idx("name"), idx(activity), typeString, idx(activity)},
		{nsA, idx("screenOrientation"), 0xffffffff, typeIntDec, uint32(orientation)},
	})
	writeStartElem("intent-filter", nil)
	writeStartElem("action", [][5]uint32{
		{nsA, idx("name"), idx("android.intent.action.MAIN"), typeString, idx("android.intent.action.MAIN")},
	})
	writeEndElem("action")
	writeStartElem("category", [][5]uint32{
		{nsA, idx("name"), idx("android.intent.category.LAUNCHER"), typeString, idx("android.intent.category.LAUNCHER")},
	})
	writeEndElem("category")
	writeEndElem("intent-filter")
	writeEndElem("activity")
	writeEndElem("application")
	writeEndElem("manifest")

	var out bytes.Buffer
	total := uint32(8 + len(sp) + body.Len())
	writeU16(&out, resXMLType)
	writeU16(&out, 8)
	writeU32(&out, total)
	out.Write(sp)
	out.Write(body.Bytes())
	return out.Bytes()
}

func buildUTF8StringPool(strs []string) []byte {
	var data bytes.Buffer
	offsets := make([]uint32, len(strs))
	for i, s := range strs {
		offsets[i] = uint32(data.Len())
		data.WriteByte(byte(len([]rune(s))))
		data.WriteByte(byte(len(s)))
		data.WriteString(s)
		data.WriteByte(0)
	}
	for data.Len()%4 != 0 {
		data.WriteByte(0)
	}
	stringsStart := uint32(28 + len(strs)*4)
	chunkSize := stringsStart + uint32(data.Len())
	var out bytes.Buffer
	writeU16(&out, resStringPoolType)
	writeU16(&out, 28)
	writeU32(&out, chunkSize)
	writeU32(&out, uint32(len(strs)))
	writeU32(&out, 0)     // styleCount
	writeU32(&out, 0x100) // UTF-8 flag
	writeU32(&out, stringsStart)
	writeU32(&out, 0)
	for _, off := range offsets {
		writeU32(&out, off)
	}
	out.Write(data.Bytes())
	return out.Bytes()
}

func writeU16(b *bytes.Buffer, v uint16) {
	var tmp [2]byte
	binary.LittleEndian.PutUint16(tmp[:], v)
	b.Write(tmp[:])
}

func writeU32(b *bytes.Buffer, v uint32) {
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], v)
	b.Write(tmp[:])
}

func writeZipFile(t *testing.T, path string, entries map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestParseSingleAPKAndSplitBundle(t *testing.T) {
	dir := t.TempDir()
	fakePNG := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0x42}, 128)...)

	apkPath := filepath.Join(dir, "demo.apk")
	writeZipFile(t, apkPath, map[string][]byte{
		"AndroidManifest.xml":                buildTestAXML("com.example.game", "Star Runner", ".MainActivity", 6),
		"res/mipmap-xxxhdpi/ic_launcher.png": fakePNG,
		"lib/arm64-v8a/libgame.so":           []byte("elf"),
	})

	meta, err := Parse(apkPath)
	if err != nil {
		t.Fatalf("Parse(.apk): %v", err)
	}
	if meta.Package != "com.example.game" || meta.Label != "Star Runner" {
		t.Fatalf("unexpected meta: %+v", meta)
	}
	if meta.VersionName != "2.4.1" || meta.VersionCode != 42 || meta.MinSDK != 24 || meta.TargetSDK != 35 {
		t.Fatalf("unexpected version/sdk: %+v", meta)
	}
	if meta.LaunchActivity != "com.example.game.MainActivity" {
		t.Fatalf("launchActivity = %q", meta.LaunchActivity)
	}
	if meta.Orientation != OrientationLandscape {
		t.Fatalf("orientation = %q, want landscape", meta.Orientation)
	}
	if !slices.Equal(meta.ABIs, []string{"arm64-v8a"}) {
		t.Fatalf("abis = %v", meta.ABIs)
	}
	if len(meta.IconBytes) != len(fakePNG) || meta.IconExt != ".png" {
		t.Fatalf("icon len=%d ext=%q", len(meta.IconBytes), meta.IconExt)
	}

	// Split bundle (.xapk) where base has manifest and split has arm64 lib.
	baseBytes, _ := os.ReadFile(apkPath)
	splitLib := filepath.Join(dir, "config.x86_64.apk")
	writeZipFile(t, splitLib, map[string][]byte{
		"lib/x86_64/libgame.so": []byte("elf"),
	})
	splitLibBytes, _ := os.ReadFile(splitLib)
	xapkPath := filepath.Join(dir, "demo.xapk")
	writeZipFile(t, xapkPath, map[string][]byte{
		"base.apk":          baseBytes,
		"config.x86_64.apk": splitLibBytes,
		"manifest.json":     []byte(`{}`),
	})
	bm, err := Parse(xapkPath)
	if err != nil {
		t.Fatalf("Parse(.xapk): %v", err)
	}
	if !bm.IsSplitBundle || !slices.Equal(bm.ABIs, []string{"arm64-v8a", "x86_64"}) {
		t.Fatalf("bundle meta = %+v", bm)
	}
}

func TestValidationAndABI(t *testing.T) {
	for _, bad := range []string{"", "../evil", "com.a; reboot", "singleword", "1bad.pkg"} {
		if ValidPackageName(bad) {
			t.Errorf("ValidPackageName(%q) = true, want false", bad)
		}
	}
	if !ValidPackageName("com.zhiliaoapp.musically") {
		t.Error("expected valid package")
	}
	if err := CheckABICompatible([]string{"armeabi-v7a"}, []string{"arm64-v8a"}); err == nil {
		t.Error("expected incompatible ABI error")
	}
	if err := CheckABICompatible(nil, []string{"arm64-v8a"}); err != nil {
		t.Errorf("universal APK should pass: %v", err)
	}
}
