// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"archive/tar"
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
