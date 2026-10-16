// SPDX-License-Identifier: AGPL-3.0-or-later

// Package snapshot writes and reads encrypted copies of a profile folder.
//
// Format: "APSNAP1\n", the chunk size (uint32, big endian), a 12-byte base
// nonce, then chunks. The plaintext is tar inside gzip. Each chunk is a
// uint32 ciphertext length and AES-256-GCM ciphertext of up to chunk-size
// bytes. Chunk i uses the base nonce with its last 8 bytes XOR i, and as
// additional data i (uint64) plus a flag byte that is 1 only on the last
// chunk. The last chunk always carries the flag, even when empty, so a cut,
// reordered or extended file fails to decrypt.
package snapshot

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/D0bby999/anyprofile/internal/engine"
)

const (
	magic     = "APSNAP1\n"
	chunkSize = 1 << 20
	// MaxUnpacked caps what a restore may write (a broken or hostile archive).
	MaxUnpacked = 20 << 30
)

// ErrCorrupt means the file is not a snapshot made with this key, or was changed.
var ErrCorrupt = errors.New("snapshot is damaged or was made with another key")

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("snapshot key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func chunkNonce(base []byte, i uint64) []byte {
	n := append([]byte(nil), base...)
	var ctr [8]byte
	binary.BigEndian.PutUint64(ctr[:], i)
	for k := 0; k < 8; k++ {
		n[4+k] ^= ctr[k]
	}
	return n
}

func chunkAAD(i uint64, last bool) []byte {
	a := make([]byte, 9)
	binary.BigEndian.PutUint64(a, i)
	if last {
		a[8] = 1
	}
	return a
}

// sealer encrypts what it is given in chunks; Close writes the last chunk.
type sealer struct {
	w     io.Writer
	gcm   cipher.AEAD
	base  []byte
	i     uint64
	buf   []byte
	n     int64
	plain int64
}

func newSealer(w io.Writer, key []byte) (*sealer, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	base := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(base); err != nil {
		return nil, err
	}
	s := &sealer{w: w, gcm: gcm, base: base}
	head := make([]byte, 0, len(magic)+4+len(base))
	head = append(head, magic...)
	head = binary.BigEndian.AppendUint32(head, chunkSize)
	head = append(head, base...)
	return s, s.out(head)
}

func (s *sealer) out(b []byte) error {
	n, err := s.w.Write(b)
	s.n += int64(n)
	return err
}

func (s *sealer) emit(plain []byte, last bool) error {
	ct := s.gcm.Seal(nil, chunkNonce(s.base, s.i), plain, chunkAAD(s.i, last))
	s.i++
	var l [4]byte
	binary.BigEndian.PutUint32(l[:], uint32(len(ct)))
	if err := s.out(l[:]); err != nil {
		return err
	}
	return s.out(ct)
}

func (s *sealer) Write(p []byte) (int, error) {
	s.buf = append(s.buf, p...)
	s.plain += int64(len(p))
	// Keep at least one byte back so the last chunk is only written by Close.
	for len(s.buf) > chunkSize {
		if err := s.emit(s.buf[:chunkSize], false); err != nil {
			return 0, err
		}
		s.buf = s.buf[chunkSize:]
	}
	return len(p), nil
}

func (s *sealer) Close() error { return s.emit(s.buf, true) }

// Write archives srcDir, skipping what skip selects, and encrypts it to w.
// It returns the number of bytes written.
func Write(w io.Writer, srcDir string, key []byte, skip func(rel string, d fs.DirEntry) bool) (int64, error) {
	s, err := newSealer(w, key)
	if err != nil {
		return 0, err
	}
	zw := gzip.NewWriter(s)
	tw := tar.NewWriter(zw)
	err = filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil || rel == "." {
			return err
		}
		rel = filepath.ToSlash(rel)
		if skip != nil && skip(rel, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		return addEntry(tw, path, rel, d)
	})
	if err == nil {
		err = tw.Close()
	}
	if err == nil {
		err = zw.Close()
	}
	if err == nil {
		err = s.Close()
	}
	return s.n, err
}

func addEntry(tw *tar.Writer, path, rel string, d fs.DirEntry) error {
	info, err := d.Info()
	if err != nil {
		return err
	}
	link := ""
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		if link, err = os.Readlink(path); err != nil {
			return err
		}
	case info.IsDir(), info.Mode().IsRegular():
	default:
		return nil // sockets and pipes (Chromium's SingletonSocket) are not data
	}
	h, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return err
	}
	h.Name = rel
	if info.IsDir() {
		h.Name += "/"
	}
	h.Uname, h.Gname, h.Uid, h.Gid = "", "", 0, 0
	if err := tw.WriteHeader(h); err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(tw, f)
	return err
}

// opener decrypts chunks in order and refuses anything out of place.
type opener struct {
	r    *bufio.Reader
	gcm  cipher.AEAD
	base []byte
	i    uint64
	buf  []byte
	done bool
}

func (o *opener) Read(p []byte) (int, error) {
	for len(o.buf) == 0 {
		if o.done {
			return 0, io.EOF
		}
		if err := o.next(); err != nil {
			return 0, err
		}
	}
	n := copy(p, o.buf)
	o.buf = o.buf[n:]
	return n, nil
}

func (o *opener) next() error {
	var l [4]byte
	if _, err := io.ReadFull(o.r, l[:]); err != nil {
		return ErrCorrupt // the last chunk never came: cut short
	}
	size := binary.BigEndian.Uint32(l[:])
	if size > chunkSize+uint32(o.gcm.Overhead()) {
		return ErrCorrupt
	}
	ct := make([]byte, size)
	if _, err := io.ReadFull(o.r, ct); err != nil {
		return ErrCorrupt
	}
	nonce := chunkNonce(o.base, o.i)
	if plain, err := o.gcm.Open(nil, nonce, ct, chunkAAD(o.i, false)); err == nil {
		o.buf, o.i = plain, o.i+1
		return nil
	}
	plain, err := o.gcm.Open(nil, nonce, ct, chunkAAD(o.i, true))
	if err != nil {
		return ErrCorrupt
	}
	if _, err := o.r.ReadByte(); err != io.EOF {
		return ErrCorrupt // data after the last chunk
	}
	o.buf, o.i, o.done = plain, o.i+1, true
	return nil
}

func newOpener(r io.Reader, key []byte) (*opener, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	br := bufio.NewReader(r)
	head := make([]byte, len(magic)+4+gcm.NonceSize())
	if _, err := io.ReadFull(br, head); err != nil || string(head[:len(magic)]) != magic {
		return nil, ErrCorrupt
	}
	if binary.BigEndian.Uint32(head[len(magic):]) != chunkSize {
		return nil, ErrCorrupt
	}
	return &opener{r: br, gcm: gcm, base: head[len(magic)+4:]}, nil
}

// Read decrypts a snapshot into dstDir. Nothing outside dstDir is written,
// links may only point inside it, and at most MaxUnpacked bytes are written.
func Read(r io.Reader, key []byte, dstDir string) error {
	o, err := newOpener(r, key)
	if err != nil {
		return err
	}
	zr, err := gzip.NewReader(o)
	if err != nil {
		return ErrCorrupt
	}
	if err := untar(tar.NewReader(zr), dstDir); err != nil {
		return err
	}
	// gzip can stop before the last chunk; read on so a missing one is caught.
	if _, err := io.Copy(io.Discard, o); err != nil {
		return err
	}
	if !o.done {
		return ErrCorrupt
	}
	return nil
}

func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func untar(tr *tar.Reader, dst string) error {
	root, err := filepath.Abs(dst)
	if err != nil {
		return err
	}
	var total int64
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(root, filepath.FromSlash(h.Name))
		if !within(root, target) || target == root {
			return fmt.Errorf("snapshot entry %q escapes the folder", h.Name)
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if engine.AbsLink(h.Linkname) || !within(root, filepath.Join(filepath.Dir(target), h.Linkname)) {
				return fmt.Errorf("snapshot link %q escapes the folder", h.Name)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			if err := os.Symlink(h.Linkname, target); err != nil {
				return err
			}
		case tar.TypeReg:
			if total += h.Size; total > MaxUnpacked {
				return fmt.Errorf("snapshot is larger than %d GiB", MaxUnpacked>>30)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(h.Mode)&0o700|0o600)
			if err != nil {
				return err
			}
			_, err = io.Copy(f, io.LimitReader(tr, h.Size))
			if cerr := f.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported snapshot entry %s (type %c)", h.Name, h.Typeflag)
		}
	}
}
