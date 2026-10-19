// SPDX-License-Identifier: AGPL-3.0-or-later

package pro

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

// Bounds on what a pack may unpack to.
const (
	maxFile  = 50 << 20
	maxTotal = 200 << 20
	maxFiles = 2000
)

// The pack lives in root ("<data>/pro"): current/ is installed, previous/
// the one before, kept for going back.
func Current(root string) string  { return filepath.Join(root, "current") }
func previous(root string) string { return filepath.Join(root, "previous") }

// Recipes is the installed pack's recipes folder.
func Recipes(root string) string { return filepath.Join(Current(root), "recipes") }

// Scripts is the installed pack's scripts folder.
func Scripts(root string) string { return filepath.Join(Current(root), "scripts") }

// Installed is the installed pack's version, "" when none.
func Installed(root string) string {
	b, err := os.ReadFile(filepath.Join(Current(root), "VERSION"))
	if err != nil {
		return ""
	}
	v := strings.TrimSpace(string(b))
	if !versionOK(v) {
		return ""
	}
	return v
}

// Install unpacks a verified pack zip as root/current. It unpacks into a
// staging folder first and swaps folders only when every file is out, so a
// failure leaves the installed pack as it was. The scripts' node_modules
// (the user's npm install) moves over, so an update does not undo it.
func Install(zipPath, version, root string) error {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	stage := filepath.Join(root, ".staging-"+version)
	os.RemoveAll(stage)
	if err := unzip(zipPath, stage); err != nil {
		os.RemoveAll(stage)
		return err
	}
	if got := readVersion(stage); got != version {
		os.RemoveAll(stage)
		return apperr.Newf("pro_bad_signature", "the pack says it is %q, not %s", got, version)
	}
	cur, prev := Current(root), previous(root)
	if nm := filepath.Join(cur, "scripts", "node_modules"); dirExists(nm) && dirExists(filepath.Join(stage, "scripts")) {
		_ = os.Rename(nm, filepath.Join(stage, "scripts", "node_modules"))
	}
	if err := os.RemoveAll(prev); err != nil {
		os.RemoveAll(stage)
		return err
	}
	if dirExists(cur) {
		if err := os.Rename(cur, prev); err != nil {
			os.RemoveAll(stage)
			return err
		}
	}
	if err := os.Rename(stage, cur); err != nil {
		_ = os.Rename(prev, cur) // put the old one back
		os.RemoveAll(stage)
		return err
	}
	return nil
}

// Remove deletes the pack from this computer.
func Remove(root string) error { return os.RemoveAll(root) }

func readVersion(dir string) string {
	b, _ := os.ReadFile(filepath.Join(dir, "VERSION"))
	return strings.TrimSpace(string(b))
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// unzip writes plain files only: no absolute or escaping paths, no links,
// within the bounds above.
func unzip(zipPath, dst string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return apperr.New("pro_bad_signature", "the pack is not a zip: "+err.Error())
	}
	defer zr.Close()
	if len(zr.File) > maxFiles {
		return apperr.Newf("pro_bad_signature", "the pack holds %d files", len(zr.File))
	}
	var total int64
	for _, f := range zr.File {
		name := f.Name
		clean := path.Clean(name)
		if name == "" || strings.Contains(name, `\`) || path.IsAbs(name) || clean == ".." || strings.HasPrefix(clean, "../") ||
			filepath.VolumeName(name) != "" {
			return apperr.Newf("pro_bad_signature", "the pack has a bad path %q", name)
		}
		mode := f.Mode()
		if mode.IsDir() || strings.HasSuffix(name, "/") {
			continue
		}
		if !mode.IsRegular() {
			return apperr.Newf("pro_bad_signature", "the pack has a link or device at %q", name)
		}
		if f.UncompressedSize64 > maxFile {
			return apperr.Newf("pro_bad_signature", "%s is larger than 50 MB", name)
		}
		out := filepath.Join(dst, filepath.FromSlash(clean))
		if err := os.MkdirAll(filepath.Dir(out), 0o700); err != nil {
			return err
		}
		n, err := extract(f, out)
		if err != nil {
			return err
		}
		if total += n; total > maxTotal {
			return errors.New("the pack unpacks to more than 200 MB")
		}
	}
	return nil
}

func extract(f *zip.File, out string) (int64, error) {
	rc, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer rc.Close()
	w, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return 0, fmt.Errorf("unpack %s: %w", f.Name, err)
	}
	n, err := io.Copy(w, io.LimitReader(rc, maxFile+1))
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > maxFile {
		err = fmt.Errorf("%s is larger than 50 MB", f.Name)
	}
	return n, err
}

func copyHashed(w io.Writer, r io.Reader) (string, int64, error) {
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(w, h), r)
	return hex.EncodeToString(h.Sum(nil)), n, err
}
