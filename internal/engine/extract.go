// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ulikunitz/xz"
)

// ExtractTarXZ unpacks a .tar.xz into dst, keeping modes and symlinks (an
// .app bundle needs both) and skipping AppleDouble metadata. Entries that
// would land outside dst are refused.
func ExtractTarXZ(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	xr, err := xz.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(xr)
	root, err := filepath.Abs(dst)
	if err != nil {
		return err
	}
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		// AppleDouble "._name" entries are macOS metadata that bsdtar folds
		// back into extended attributes. Written out as files they become
		// extra files inside a sealed bundle and break its signature.
		if strings.HasPrefix(filepath.Base(h.Name), "._") {
			continue
		}
		target := filepath.Join(root, h.Name)
		if !within(root, target) {
			return fmt.Errorf("archive entry escapes destination: %s", h.Name)
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(h.Mode)&0o755)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			if cerr := out.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
		case tar.TypeSymlink:
			// Relative links inside the bundle only.
			if filepath.IsAbs(h.Linkname) || !within(root, filepath.Join(filepath.Dir(target), h.Linkname)) {
				return fmt.Errorf("symlink escapes destination: %s -> %s", h.Name, h.Linkname)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(h.Linkname, target); err != nil {
				return err
			}
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
		default:
			return fmt.Errorf("unsupported archive entry %s (type %c)", h.Name, h.Typeflag)
		}
	}
}

// ExtractZip unpacks a .zip into dst with the same rules as ExtractTarXZ:
// modes and relative symlinks kept, AppleDouble metadata skipped, nothing
// written outside dst.
func ExtractZip(src, dst string) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()
	root, err := filepath.Abs(dst)
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "__MACOSX/") || strings.HasPrefix(filepath.Base(f.Name), "._") {
			continue
		}
		target := filepath.Join(root, f.Name)
		if !within(root, target) {
			return fmt.Errorf("zip entry %q escapes the destination", f.Name)
		}
		if err := extractZipEntry(f, root, target); err != nil {
			return err
		}
	}
	return nil
}

func extractZipEntry(f *zip.File, root, target string) error {
	mode := f.Mode()
	switch {
	case mode.IsDir():
		return os.MkdirAll(target, 0o755)
	case mode&os.ModeSymlink != 0:
		rc, err := f.Open()
		if err != nil {
			return err
		}
		b, err := io.ReadAll(io.LimitReader(rc, 4096))
		rc.Close()
		if err != nil {
			return err
		}
		link := string(b)
		if filepath.IsAbs(link) || !within(root, filepath.Join(filepath.Dir(target), link)) {
			return fmt.Errorf("zip symlink %q escapes the destination", f.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.Symlink(link, target)
	case mode.IsRegular():
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm()&0o755)
		if err != nil {
			return err
		}
		n, err := io.Copy(out, io.LimitReader(rc, int64(f.UncompressedSize64)+1))
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err == nil && uint64(n) > f.UncompressedSize64 {
			err = fmt.Errorf("zip entry %q is larger than declared", f.Name)
		}
		return err
	default:
		return fmt.Errorf("unsupported zip entry %s (mode %v)", f.Name, mode)
	}
}

func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// VerifyCodesign checks an app bundle's signature is intact and made by team.
func VerifyCodesign(app, team string) error {
	if out, err := exec.Command("/usr/bin/codesign", "--verify", "--deep", "--strict", app).CombinedOutput(); err != nil {
		return fmt.Errorf("codesign verify %s: %v: %s", app, err, bytes.TrimSpace(out))
	}
	out, err := exec.Command("/usr/bin/codesign", "-dv", app).CombinedOutput()
	if err != nil {
		return fmt.Errorf("codesign -dv: %v", err)
	}
	if !bytes.Contains(out, []byte("\nTeamIdentifier="+team+"\n")) {
		return fmt.Errorf("%s is not signed by team %s", app, team)
	}
	return nil
}
