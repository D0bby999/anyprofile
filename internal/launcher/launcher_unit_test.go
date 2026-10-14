// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

func TestCookieKeyLifecycle(t *testing.T) {
	sec := secrets.NewMemory()
	udd := t.TempDir()

	k1, err := cookieKey(sec, "p", udd)
	if err != nil || len(k1) != 32 {
		t.Fatalf("first key: %v (%d bytes)", err, len(k1))
	}
	k2, err := cookieKey(sec, "p", udd)
	if err != nil || !bytes.Equal(k1, k2) {
		t.Fatal("key must be stable across launches")
	}

	// A different key in the Keychain than the profile was sealed with: refuse.
	sec.Set(secrets.ServiceCookieKey, "p", bytes.Repeat([]byte{1}, 32))
	if _, err := cookieKey(sec, "p", udd); err == nil {
		t.Fatal("mismatched key accepted")
	}

	// Key gone but the profile has a fingerprint: refuse, never regenerate silently.
	sec.Delete(secrets.ServiceCookieKey, "p")
	if _, err := cookieKey(sec, "p", udd); !errors.Is(err, ErrCookieKeyLost) {
		t.Fatalf("lost key err = %v", err)
	}

	// Key present, fingerprint missing (older profile): adopt the key and record it.
	udd2 := t.TempDir()
	sec.Set(secrets.ServiceCookieKey, "q", k1)
	if k, err := cookieKey(sec, "q", udd2); err != nil || !bytes.Equal(k, k1) {
		t.Fatalf("existing key: %v", err)
	}
	if _, err := os.Stat(filepath.Join(udd2, keyFingerprintFile)); err != nil {
		t.Fatal("fingerprint not recorded")
	}

	sec.Set(secrets.ServiceCookieKey, "r", []byte("short"))
	if _, err := cookieKey(sec, "r", t.TempDir()); err == nil {
		t.Fatal("short key accepted")
	}
}

func TestSingletonPID(t *testing.T) {
	udd := t.TempDir()
	if singletonPID(udd) != 0 {
		t.Fatal("no lock must read as 0")
	}
	os.Symlink("my-mac.local-4242", filepath.Join(udd, "SingletonLock"))
	if got := singletonPID(udd); got != 4242 {
		t.Fatalf("pid = %d", got)
	}
	if !alive(os.Getpid()) || sameProcess(os.Getpid(), "0.0") {
		t.Fatal("alive/sameProcess wrong for this process")
	}
	st, err := pidStartTime(os.Getpid())
	if err != nil || !sameProcess(os.Getpid(), st) {
		t.Fatalf("start time: %q %v", st, err)
	}
}

// stubEngine stops every launch at BuildLaunch, after the cookie-key step.
type stubEngine struct {
	engine.Engine
	cookieKey bool
}

func (stubEngine) ID() string { return "stub" }
func (stubEngine) Status(context.Context) engine.Status {
	return engine.Status{Installed: true}
}
func (stubEngine) PrepareProfile(store.Profile, engine.Runtime) error { return nil }
func (e stubEngine) UsesCookieKey() bool                              { return e.cookieKey }
func (stubEngine) BuildLaunch(context.Context, store.Profile, engine.Runtime) (engine.LaunchSpec, error) {
	return engine.LaunchSpec{}, errors.New("stop here")
}

func TestCookieKeyOnlyWhenEngineUsesIt(t *testing.T) {
	for _, uses := range []bool{false, true} {
		dirs := paths.Dirs{Root: t.TempDir()}
		if err := dirs.Ensure(); err != nil {
			t.Fatal(err)
		}
		st, err := store.Open(dirs.DB(), dirs.Backups())
		if err != nil {
			t.Fatal(err)
		}
		p := &store.Profile{ID: "p1", Name: "P", Engine: "stub", EngineVersion: "1", PersonaID: "x",
			SeedHex: "00112233aabbccdd", Platform: "macos", Timezone: "UTC", Languages: "en-US", UILocale: "en-US"}
		if err := st.CreateProfile(p); err != nil {
			t.Fatal(err)
		}
		sec := secrets.NewMemory()
		l := New(st, sec, dirs, []engine.Engine{stubEngine{cookieKey: uses}}, nil,
			slog.New(slog.NewTextHandler(io.Discard, nil)))
		if err := l.Start(context.Background(), "p1"); err == nil || err.Error() != "stop here" {
			t.Fatalf("start err = %v", err)
		}
		_, err = sec.Get(secrets.ServiceCookieKey, "p1")
		if has := err == nil; has != uses {
			t.Fatalf("uses=%v: key stored = %v", uses, has)
		}
		st.Close()
	}
}
