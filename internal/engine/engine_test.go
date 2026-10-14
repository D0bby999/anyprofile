// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ulikunitz/xz"
)

func makeTarXZ(t *testing.T, add func(tw *tar.Writer)) []byte {
	t.Helper()
	var buf bytes.Buffer
	xw, err := xz.NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(xw)
	add(tw)
	tw.Close()
	xw.Close()
	return buf.Bytes()
}

func file(tw *tar.Writer, name, body string, mode int64) {
	tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: mode, Size: int64(len(body))})
	tw.Write([]byte(body))
}

func TestExtractKeepsModesAndSymlinks(t *testing.T) {
	data := makeTarXZ(t, func(tw *tar.Writer) {
		tw.WriteHeader(&tar.Header{Name: "X.app/Contents/", Typeflag: tar.TypeDir, Mode: 0o755})
		file(tw, "X.app/Contents/MacOS/X", "bin", 0o755)
		file(tw, "X.app/Contents/MacOS/._X", "appledouble", 0o644)
		tw.WriteHeader(&tar.Header{Name: "X.app/Contents/Current", Typeflag: tar.TypeSymlink, Linkname: "MacOS"})
	})
	dir := t.TempDir()
	src := filepath.Join(dir, "a.tar.xz")
	os.WriteFile(src, data, 0o600)
	dst := filepath.Join(dir, "out")
	if err := ExtractTarXZ(src, dst); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dst, "X.app/Contents/MacOS/X"))
	if err != nil || fi.Mode().Perm()&0o100 == 0 {
		t.Fatalf("binary not executable: %v %v", fi, err)
	}
	if l, err := os.Readlink(filepath.Join(dst, "X.app/Contents/Current")); err != nil || l != "MacOS" {
		t.Fatalf("symlink = %q, %v", l, err)
	}
	if _, err := os.Stat(filepath.Join(dst, "X.app/Contents/MacOS/._X")); !os.IsNotExist(err) {
		t.Fatal("AppleDouble entry written into the bundle")
	}
}

func TestExtractRefusesEscapes(t *testing.T) {
	for name, add := range map[string]func(*tar.Writer){
		"dotdot": func(tw *tar.Writer) { file(tw, "../evil", "x", 0o644) },
		"symlink": func(tw *tar.Writer) {
			tw.WriteHeader(&tar.Header{Name: "a/link", Typeflag: tar.TypeSymlink, Linkname: "../../etc"})
		},
	} {
		dir := t.TempDir()
		src := filepath.Join(dir, "a.tar.xz")
		os.WriteFile(src, makeTarXZ(t, add), 0o600)
		if err := ExtractTarXZ(src, filepath.Join(dir, "out")); err == nil || !strings.Contains(err.Error(), "escapes") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestDownloadResumesAndVerifies(t *testing.T) {
	body := bytes.Repeat([]byte("fury"), 50000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "f", time.Time{}, bytes.NewReader(body))
	}))
	defer srv.Close()
	dir := t.TempDir()
	dst := filepath.Join(dir, "f")
	os.WriteFile(dst+".part", body[:1000], 0o600) // an interrupted earlier attempt
	var last int64
	if err := Download(context.Background(), srv.URL, dst, func(d, _ int64) { last = d }); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dst)
	if !bytes.Equal(got, body) || last != int64(len(body)) {
		t.Fatalf("resumed file wrong: %d bytes, progress %d", len(got), last)
	}
	sum, _ := SHA256File(dst)
	if VerifySHA256(dst, sum) != nil || VerifySHA256(dst, strings.Repeat("0", 64)) == nil {
		t.Fatal("hash verification wrong")
	}
}

type zipEntry struct {
	name, body string
	mode       os.FileMode
}

func writeZip(t *testing.T, entries []zipEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "x.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		h.SetMode(e.mode)
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return path
}

func TestExtractZip(t *testing.T) {
	src := writeZip(t, []zipEntry{
		{name: "B.framework/", mode: os.ModeDir | 0o755},
		{name: "B.framework/Versions/A/bin", body: "#!/bin/sh\n", mode: 0o755},
		{name: "B.framework/Versions/Current", body: "A", mode: os.ModeSymlink | 0o777},
		{name: "__MACOSX/x", body: "junk", mode: 0o644},
		{name: "B.framework/._y", body: "junk", mode: 0o644},
	})
	dst := t.TempDir()
	if err := ExtractZip(src, dst); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(dst, "B.framework/Versions/A/bin"))
	if err != nil || st.Mode().Perm() != 0o755 {
		t.Fatalf("bin = %v %v", st, err)
	}
	if l, err := os.Readlink(filepath.Join(dst, "B.framework/Versions/Current")); err != nil || l != "A" {
		t.Fatalf("link = %q %v", l, err)
	}
	for _, junk := range []string{"__MACOSX/x", "B.framework/._y"} {
		if _, err := os.Lstat(filepath.Join(dst, junk)); !os.IsNotExist(err) {
			t.Fatalf("%s extracted", junk)
		}
	}
}

func TestExtractZipRejects(t *testing.T) {
	cases := map[string]zipEntry{
		"dotdot":      {name: "../evil", body: "x", mode: 0o644},
		"abs symlink": {name: "a/link", body: "/etc", mode: os.ModeSymlink | 0o777},
		"up symlink":  {name: "a/link", body: "../../evil", mode: os.ModeSymlink | 0o777},
	}
	for name, e := range cases {
		parent := t.TempDir()
		dst := filepath.Join(parent, "dst")
		if err := os.Mkdir(dst, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := ExtractZip(writeZip(t, []zipEntry{e}), dst); err == nil {
			t.Fatalf("%s: accepted", name)
		}
		entries, _ := os.ReadDir(parent)
		if len(entries) != 1 {
			t.Fatalf("%s: wrote outside dst: %v", name, entries)
		}
	}
}
