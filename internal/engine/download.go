// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Download fetches url into dst, resuming a partial dst.part with a Range
// request, retrying up to three times. dst appears only once complete.
func Download(ctx context.Context, url, dst string, progress Progress) error {
	part := dst + ".part"
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			}
		}
		if last = fetch(ctx, url, part, progress); last == nil {
			return os.Rename(part, dst)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return fmt.Errorf("download %s: %w", url, last)
}

func fetch(ctx context.Context, url, part string, progress Progress) error {
	f, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	have, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if have > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(have, 10)+"-")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusPartialContent:
	case resp.StatusCode == http.StatusOK:
		// Server ignored Range: start over.
		if err := f.Truncate(0); err != nil {
			return err
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		have = 0
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		return nil // already complete; the hash check decides
	default:
		return fmt.Errorf("HTTP %s", resp.Status)
	}
	total := int64(-1)
	if resp.ContentLength >= 0 {
		total = have + resp.ContentLength
	}
	w := &progressWriter{w: f, done: have, total: total, fn: progress}
	_, err = io.Copy(w, resp.Body)
	return err
}

type progressWriter struct {
	w           io.Writer
	done, total int64
	fn          Progress
	last        time.Time
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.done += int64(n)
	if p.fn != nil && (time.Since(p.last) > 200*time.Millisecond || p.done == p.total) {
		p.last = time.Now()
		p.fn(p.done, p.total)
	}
	return n, err
}

// SHA256File returns the hex digest of a file.
func SHA256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifySHA256 fails unless path hashes to want.
func VerifySHA256(path, want string) error {
	got, err := SHA256File(path)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("sha256 mismatch for %s: got %s, want %s", path, got, want)
	}
	return nil
}
