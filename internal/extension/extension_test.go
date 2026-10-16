// SPDX-License-Identifier: AGPL-3.0-or-later

package extension

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const manifest = `{"manifest_version": 3, "name": "Helper", "version": "1.2"}`

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	zw.Close()
	return buf.Bytes()
}

func write(t *testing.T, name string, b []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestUnpackFolderZipAndCRX(t *testing.T) {
	folder := t.TempDir()
	os.WriteFile(filepath.Join(folder, "manifest.json"), []byte(manifest), 0o600)
	os.MkdirAll(filepath.Join(folder, "js"), 0o700)
	os.WriteFile(filepath.Join(folder, "js", "a.js"), []byte("1"), 0o600)

	z := zipOf(t, map[string]string{"manifest.json": manifest, "js/a.js": "1"})
	nested := zipOf(t, map[string]string{"helper-1.2/manifest.json": manifest, "helper-1.2/js/a.js": "1"})
	header := []byte("protobuf-header")
	crx := append([]byte("Cr24"), make([]byte, 8)...)
	binary.LittleEndian.PutUint32(crx[4:8], 3)
	binary.LittleEndian.PutUint32(crx[8:12], uint32(len(header)))
	crx = append(append(crx, header...), z...)

	for name, src := range map[string]string{
		"folder": folder, "zip": write(t, "x.zip", z), "zip with a top folder": write(t, "n.zip", nested), "crx3": write(t, "x.crx", crx),
	} {
		dst := filepath.Join(t.TempDir(), "lib", "e1")
		m, err := Unpack(src, dst)
		if err != nil || m != (Manifest{Name: "Helper", Version: "1.2"}) {
			t.Fatalf("%s: %+v, %v", name, m, err)
		}
		if b, err := os.ReadFile(filepath.Join(dst, "js", "a.js")); err != nil || string(b) != "1" {
			t.Fatalf("%s: js/a.js = %q, %v", name, b, err)
		}
	}
}

func TestUnpackRefusesUnsafeOrBrokenInput(t *testing.T) {
	link := t.TempDir()
	os.WriteFile(filepath.Join(link, "manifest.json"), []byte(manifest), 0o600)
	// Windows without Developer Mode cannot make symlinks; that case is then moot.
	linked := os.Symlink("/etc/passwd", filepath.Join(link, "passwd")) == nil

	cases := map[string]string{
		"zip slip":     write(t, "s.zip", zipOf(t, map[string]string{"manifest.json": manifest, "../../evil.js": "x"})),
		"absolute":     write(t, "a.zip", zipOf(t, map[string]string{"manifest.json": manifest, "/tmp/evil.js": "x"})),
		"drive":        write(t, "d.zip", zipOf(t, map[string]string{"manifest.json": manifest, "C:/evil.js": "x"})),
		"stream":       write(t, "c.zip", zipOf(t, map[string]string{"manifest.json": manifest, "a.js:evil": "x"})),
		"no manifest":  write(t, "m.zip", zipOf(t, map[string]string{"a.js": "1"})),
		"bad manifest": write(t, "b.zip", zipOf(t, map[string]string{"manifest.json": `{"manifest_version": 1, "name": "x", "version": "1"}`})),
		"not a zip":    write(t, "n.zip", []byte("hello")),
	}
	if linked {
		cases["symlink"] = link
	}
	for name, src := range cases {
		dst := filepath.Join(t.TempDir(), "e1")
		if _, err := Unpack(src, dst); err == nil {
			t.Errorf("%s: accepted", name)
		}
		if _, err := os.Stat(dst); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s: left %s behind", name, dst)
		}
	}
	if _, err := os.Stat(filepath.Join(os.TempDir(), "evil.js")); err == nil {
		t.Fatal("zip slip wrote outside the folder")
	}
}

func TestLocalizedName(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"manifest_version": 3, "name": "__MSG_appName__", "version": "2", "default_locale": "en"}`), 0o600)
	os.MkdirAll(filepath.Join(dir, "_locales", "en"), 0o700)
	os.WriteFile(filepath.Join(dir, "_locales", "en", "messages.json"), []byte(`{"appName": {"message": "Grammar Helper"}}`), 0o600)
	if m, err := ReadManifest(dir); err != nil || m.Name != "Grammar Helper" {
		t.Fatalf("name = %+v, %v", m, err)
	}
}
