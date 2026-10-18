// SPDX-License-Identifier: AGPL-3.0-or-later

package snapshot

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func key(t *testing.T) []byte {
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return k
}

func makeTree(t *testing.T) string {
	src := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(src, "Default", "Cache"), 0o700))
	must(os.MkdirAll(filepath.Join(src, "Empty"), 0o700))
	must(os.WriteFile(filepath.Join(src, "Default", "Cookies"), []byte("cookie-db"), 0o600))
	must(os.WriteFile(filepath.Join(src, "Default", "Cache", "x"), []byte("cache"), 0o600))
	big := make([]byte, 3<<20)
	rand.Read(big)
	must(os.WriteFile(filepath.Join(src, "Default", "Big"), big, 0o600))
	must(os.Symlink("Default/Cookies", filepath.Join(src, "link")))
	return src
}

func sums(t *testing.T, root string) map[string]string {
	out := map[string]string{}
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		rel, _ := filepath.Rel(root, p)
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			l, _ := os.Readlink(p)
			out[rel] = "->" + l
		case d.IsDir():
			out[rel] = "dir"
		default:
			b, _ := os.ReadFile(p)
			out[rel] = fmt.Sprintf("%x", sha256.Sum256(b))
		}
		return nil
	})
	return out
}

func snap(t *testing.T, src string, k []byte, skip func(string, fs.DirEntry) bool) []byte {
	var buf bytes.Buffer
	n, err := Write(&buf, src, k, skip)
	if err != nil || n != int64(buf.Len()) {
		t.Fatalf("write = %d %v (buffer %d)", n, err, buf.Len())
	}
	return buf.Bytes()
}

func TestRoundTrip(t *testing.T) {
	src, k := makeTree(t), key(t)
	data := snap(t, src, k, nil)
	dst := t.TempDir()
	if err := Read(bytes.NewReader(data), k, dst); err != nil {
		t.Fatal(err)
	}
	a, b := sums(t, src), sums(t, dst)
	if len(a) != len(b) {
		t.Fatalf("entries %d vs %d", len(a), len(b))
	}
	for p, s := range a {
		if b[p] != s {
			t.Errorf("%s differs", p)
		}
	}
}

func TestSkip(t *testing.T) {
	src, k := makeTree(t), key(t)
	data := snap(t, src, k, func(rel string, d fs.DirEntry) bool { return rel == "Default/Cache" })
	dst := t.TempDir()
	if err := Read(bytes.NewReader(data), k, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "Default", "Cache")); !os.IsNotExist(err) {
		t.Fatal("skipped folder was restored")
	}
	if _, err := os.Stat(filepath.Join(dst, "Default", "Cookies")); err != nil {
		t.Fatal("kept file missing")
	}
}

func TestRejectsTampering(t *testing.T) {
	src, k := makeTree(t), key(t)
	data := snap(t, src, k, nil)
	firstChunk := len(magic) + 4 + 12
	cases := map[string][]byte{
		"wrong key":      nil,
		"flipped byte":   append([]byte(nil), data...),
		"cut short":      data[:len(data)-100],
		"last chunk cut": nil,
		"chunks swapped": nil,
		"trailing data":  append(append([]byte(nil), data...), 1, 2, 3),
	}
	cases["flipped byte"][len(data)/2] ^= 1
	// Drop the final chunk entirely: find it by walking the lengths.
	var offs []int
	for off := firstChunk; off < len(data); {
		offs = append(offs, off)
		off += 4 + int(uint32(data[off])<<24|uint32(data[off+1])<<16|uint32(data[off+2])<<8|uint32(data[off+3]))
	}
	cases["last chunk cut"] = data[:offs[len(offs)-1]]
	if len(offs) < 3 {
		t.Fatalf("want several chunks, got %d", len(offs))
	}
	sw := append([]byte(nil), data[:offs[0]]...)
	sw = append(sw, data[offs[1]:offs[2]]...)
	sw = append(sw, data[offs[0]:offs[1]]...)
	sw = append(sw, data[offs[2]:]...)
	cases["chunks swapped"] = sw
	for name, d := range cases {
		k2 := k
		if name == "wrong key" {
			d, k2 = data, key(t)
		}
		if err := Read(bytes.NewReader(d), k2, t.TempDir()); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestRejectsEscapes(t *testing.T) {
	k := key(t)
	for name, h := range map[string]*tar.Header{
		"dotdot":  {Name: "../evil", Typeflag: tar.TypeReg, Size: 1, Mode: 0o600},
		"symlink": {Name: "link", Typeflag: tar.TypeSymlink, Linkname: "../../etc"},
	} {
		var plain bytes.Buffer
		zw := gzip.NewWriter(&plain)
		tw := tar.NewWriter(zw)
		tw.WriteHeader(h)
		if h.Size > 0 {
			tw.Write([]byte("x"))
		}
		tw.Close()
		zw.Close()
		var out bytes.Buffer
		s, err := newSealer(&out, k)
		if err != nil {
			t.Fatal(err)
		}
		s.Write(plain.Bytes())
		s.Close()
		parent := t.TempDir()
		dst := filepath.Join(parent, "dst")
		os.Mkdir(dst, 0o700)
		if err := Read(&out, k, dst); err == nil || !strings.Contains(err.Error(), "escapes") {
			t.Errorf("%s: err = %v", name, err)
		}
		if entries, _ := os.ReadDir(parent); len(entries) != 1 {
			t.Errorf("%s: wrote outside the folder", name)
		}
	}
}

// An archive of folders under prefixes and files from memory; kept files
// come back in memory and are not written.
func TestArchiveKeeping(t *testing.T) {
	src, k := makeTree(t), key(t)
	var buf bytes.Buffer
	a, err := NewArchive(&buf, k)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.AddFile("secrets.json", []byte(`{"x":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := a.AddDir(src, "profiles/p1", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Close(); err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	kept, err := ReadKeeping(bytes.NewReader(buf.Bytes()), k, dst, func(n string) bool { return n == "secrets.json" })
	if err != nil {
		t.Fatal(err)
	}
	if string(kept["secrets.json"]) != `{"x":1}` {
		t.Fatalf("kept = %q", kept)
	}
	if _, err := os.Stat(filepath.Join(dst, "secrets.json")); !os.IsNotExist(err) {
		t.Fatal("a kept file was written to disk")
	}
	a2, b := sums(t, src), sums(t, filepath.Join(dst, "profiles", "p1"))
	if len(a2) != len(b) {
		t.Fatalf("entries %d vs %d", len(a2), len(b))
	}
}
