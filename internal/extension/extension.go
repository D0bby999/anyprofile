// SPDX-License-Identifier: AGPL-3.0-or-later

// Package extension unpacks a browser extension (a folder, a .zip or a .crx)
// into the library and reads its manifest.
package extension

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Limits keep a broken or hostile archive from filling the disk.
const (
	maxBytes = 100 << 20
	maxFiles = 10000
)

var ErrNoManifest = errors.New("no valid manifest.json")

// Manifest is the part of manifest.json the app shows.
type Manifest struct {
	Name    string
	Version string
}

// Unpack copies or extracts src into dst (which must not exist yet) and
// returns the manifest. dst is removed again on any error.
func Unpack(src, dst string) (m Manifest, err error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return m, err
	}
	if err := os.Mkdir(dst, 0o700); err != nil {
		return m, err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(dst)
		}
	}()
	st, err := os.Stat(src)
	if err != nil {
		return m, err
	}
	if st.IsDir() {
		err = copyDir(src, dst)
	} else {
		err = extractFile(src, dst)
	}
	if err != nil {
		return m, err
	}
	// A zip made by zipping the folder has it one level down.
	if _, e := os.Stat(filepath.Join(dst, "manifest.json")); errors.Is(e, fs.ErrNotExist) {
		if err := hoistSingleDir(dst); err != nil {
			return m, err
		}
	}
	return ReadManifest(dst)
}

// ReadManifest reads name and version, resolving a __MSG_x__ name from the
// default locale.
func ReadManifest(dir string) (Manifest, error) {
	var raw struct {
		ManifestVersion int    `json:"manifest_version"`
		Name            string `json:"name"`
		Version         string `json:"version"`
		DefaultLocale   string `json:"default_locale"`
	}
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return Manifest{}, ErrNoManifest
	}
	if err := json.Unmarshal(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")), &raw); err != nil ||
		raw.Name == "" || raw.Version == "" || (raw.ManifestVersion != 2 && raw.ManifestVersion != 3) {
		return Manifest{}, ErrNoManifest
	}
	name := raw.Name
	if key, ok := strings.CutPrefix(name, "__MSG_"); ok && raw.DefaultLocale != "" {
		key = strings.ToLower(strings.TrimSuffix(key, "__"))
		var msgs map[string]struct {
			Message string `json:"message"`
		}
		if b, err := os.ReadFile(filepath.Join(dir, "_locales", filepath.Base(raw.DefaultLocale), "messages.json")); err == nil {
			json.Unmarshal(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")), &msgs)
			for k, v := range msgs {
				if strings.ToLower(k) == key && v.Message != "" {
					name = v.Message
				}
			}
		}
	}
	return Manifest{Name: name, Version: raw.Version}, nil
}

func extractFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if bytes.HasPrefix(b, []byte("Cr24")) {
		if b, err = stripCRX(b); err != nil {
			return err
		}
	}
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return fmt.Errorf("not a zip or crx file: %w", err)
	}
	if len(zr.File) > maxFiles {
		return fmt.Errorf("more than %d files", maxFiles)
	}
	var total int64
	for _, f := range zr.File {
		name := path.Clean(strings.ReplaceAll(f.Name, `\`, "/"))
		if name == "." || strings.HasPrefix(name, "../") || name == ".." || path.IsAbs(name) {
			return fmt.Errorf("unsafe path %q in the archive", f.Name)
		}
		target := filepath.Join(dst, filepath.FromSlash(name))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
			continue
		}
		if !f.Mode().IsRegular() {
			return fmt.Errorf("%q is not a regular file", f.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		n, err := writeLimited(f, target, maxBytes-total)
		if err != nil {
			return err
		}
		total += n
	}
	return nil
}

func writeLimited(f *zip.File, target string, left int64) (int64, error) {
	r, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer r.Close()
	w, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(w, io.LimitReader(r, left+1))
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > left {
		err = fmt.Errorf("the extension is larger than %d MB", maxBytes>>20)
	}
	return n, err
}

// stripCRX drops the CRX2 or CRX3 header in front of the zip.
func stripCRX(b []byte) ([]byte, error) {
	if len(b) < 16 {
		return nil, errors.New("truncated crx file")
	}
	var skip uint64
	switch binary.LittleEndian.Uint32(b[4:8]) {
	case 2:
		skip = 16 + uint64(binary.LittleEndian.Uint32(b[8:12])) + uint64(binary.LittleEndian.Uint32(b[12:16]))
	case 3:
		skip = 12 + uint64(binary.LittleEndian.Uint32(b[8:12]))
	default:
		return nil, errors.New("unknown crx version")
	}
	if skip > uint64(len(b)) {
		return nil, errors.New("truncated crx file")
	}
	return b[skip:], nil
}

func copyDir(src, dst string) error {
	var total int64
	files := 0
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o700)
		case !d.Type().IsRegular():
			return fmt.Errorf("%s is not a regular file", rel)
		}
		if files++; files > maxFiles {
			return fmt.Errorf("more than %d files", maxFiles)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if total += int64(len(b)); total > maxBytes {
			return fmt.Errorf("the extension is larger than %d MB", maxBytes>>20)
		}
		return os.WriteFile(target, b, 0o600)
	})
}

// hoistSingleDir moves the content of dst/<only dir> up into dst.
func hoistSingleDir(dst string) error {
	es, err := os.ReadDir(dst)
	if err != nil {
		return err
	}
	if len(es) != 1 || !es[0].IsDir() {
		return ErrNoManifest
	}
	inner := filepath.Join(dst, es[0].Name())
	kids, err := os.ReadDir(inner)
	if err != nil {
		return err
	}
	for _, k := range kids {
		if err := os.Rename(filepath.Join(inner, k.Name()), filepath.Join(dst, k.Name())); err != nil {
			return err
		}
	}
	return os.Remove(inner)
}
