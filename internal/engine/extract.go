// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"archive/tar"
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
