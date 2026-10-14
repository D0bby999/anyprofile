// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadCookieFile(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "c.txt")
	os.WriteFile(good, []byte(".example.com\tTRUE\t/\tFALSE\t0\tlang\ten\n"), 0o600)
	if cs, _, err := readCookieFile(good); err != nil || len(cs) != 1 {
		t.Fatalf("good file = %d %v", len(cs), err)
	}
	big := filepath.Join(dir, "big.txt")
	os.WriteFile(big, make([]byte, 5<<20+1), 0o600)
	if _, _, err := readCookieFile(big); err == nil || !strings.Contains(err.Error(), "cookies_file_too_big") {
		t.Fatalf("big file err = %v", err)
	}
	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte("[{"), 0o600)
	if _, _, err := readCookieFile(bad); err == nil || !strings.Contains(err.Error(), "cookies_bad_format") {
		t.Fatalf("bad file err = %v", err)
	}
}

func TestSiteMismatch(t *testing.T) {
	tiktok := []string{"tiktok.com", "tiktokv.com"}
	for top, want := range map[string]string{"tiktok.com": "", "studio.tiktok.com": "", "youtube.com": "youtube.com", "": ""} {
		if got := siteMismatch(top, tiktok); got != want {
			t.Errorf("siteMismatch(%q) = %q, want %q", top, got, want)
		}
	}
	if got := siteMismatch("youtube.com", nil); got != "" {
		t.Errorf("platform Other must not warn: %q", got)
	}
}
