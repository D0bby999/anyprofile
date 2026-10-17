// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package launcher

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/secrets"
	_ "modernc.org/sqlite"
)

const canvasHash = `(async () => {
  const c = document.createElement('canvas'); c.width = 240; c.height = 60;
  const g = c.getContext('2d');
  g.textBaseline = 'top'; g.font = '16px Arial'; g.fillStyle = '#f60'; g.fillRect(10, 10, 100, 30);
  g.fillStyle = '#069'; g.fillText('AnyProfile canvas 1.0', 4, 20);
  const buf = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(c.toDataURL()));
  return [...new Uint8Array(buf)].slice(0, 8).map(b => b.toString(16).padStart(2, '0')).join('');
})()`

// A profile closed and reopened keeps its login cookie and draws the same
// canvas; the cookie is stored encrypted, and the window carries the name.
func TestSessionAndFingerprintSurviveRestart(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "anyprofile-secret-cookie", MaxAge: 86400, Path: "/"})
		}
		fmt.Fprint(w, "<!doctype html><title>AP test</title><body>ok</body>")
	}))
	defer srv.Close()

	r := newRig(t)
	l := r.launcher(t)
	id := r.profile(t, 3)

	open := func() *cdp {
		p, _ := r.st.GetProfile(id)
		p.CDPNextLaunch = true
		if err := r.st.UpdateProfile(p); err != nil {
			t.Fatal(err)
		}
		if err := l.StartBehind(context.Background(), id); err != nil {
			t.Fatal(err)
		}
		return dialPage(t, l.List()[0].CDPPort)
	}

	d := open()
	d.navigate(t, srv.URL+"/login")
	hash1 := d.eval(t, canvasHash)
	title := d.eval(t, "document.title")
	d.close()
	t0 := time.Now()
	if err := l.Stop(id); err != nil {
		t.Fatal(err)
	}
	t.Logf("stop took %v", time.Since(t0))
	if os.Getenv("AP_DEBUG") != "" {
		out, _ := exec.Command("sh", "-c", "cd "+r.dirs.Profile(id)+" && ls -la Default/Cookies* Default/Network/Cookies* 2>&1; tail -5 "+filepath.Join(r.dirs.Logs(), "browser-"+id+".log")).CombinedOutput()
		t.Logf("\n%s", out)
	}

	// On disk the cookie value is encrypted, never plaintext.
	db, err := sql.Open("sqlite", "file:"+filepath.Join(r.dirs.Profile(id), "Default", "Cookies")+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	var plain string
	var enc []byte
	err = db.QueryRow(`SELECT value, encrypted_value FROM cookies WHERE name = 'session'`).Scan(&plain, &enc)
	db.Close()
	if err != nil {
		t.Fatalf("cookie row: %v", err)
	}
	if plain != "" || len(enc) == 0 || strings.Contains(string(enc), "anyprofile-secret-cookie") {
		t.Fatalf("cookie not encrypted at rest: value=%q enc=%d bytes", plain, len(enc))
	}

	d = open()
	d.navigate(t, srv.URL+"/check")
	if cookie := d.eval(t, "document.cookie"); !strings.Contains(cookie, "session=anyprofile-secret-cookie") {
		t.Fatalf("session lost after restart: %q", cookie)
	}
	if hash2 := d.eval(t, canvasHash); hash2 != hash1 || hash1 == "" {
		t.Fatalf("canvas hash changed across restart: %s -> %s", hash1, hash2)
	}
	d.close()
	l.Stop(id)
	t.Logf("canvas hash %s, page title %q", hash1, title)

	// Proof the jar is sealed with this profile's own key: swap the key (and
	// drop the fingerprint guard that would normally refuse this) and the
	// cookie is unreadable.
	r.sec.Set(secrets.ServiceCookieKey, id, bytes.Repeat([]byte{7}, 32))
	os.Remove(filepath.Join(r.dirs.Profile(id), keyFingerprintFile))
	d = open()
	defer func() { d.close(); l.Stop(id) }()
	d.navigate(t, srv.URL+"/check")
	if cookie := d.eval(t, "document.cookie"); strings.Contains(cookie, "anyprofile-secret-cookie") {
		t.Fatalf("cookie readable with a different key: the per-profile key is not in use")
	}
}
