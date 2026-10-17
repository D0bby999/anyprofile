// SPDX-License-Identifier: AGPL-3.0-or-later

package secrets

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// countingStore counts reads of the system store, which is what asks the
// user on macOS.
type countingStore struct {
	*Memory
	gets map[string]int
	fail error
}

func newCounting() *countingStore { return &countingStore{Memory: NewMemory(), gets: map[string]int{}} }

func (s *countingStore) Get(service, account string) ([]byte, error) {
	s.gets[service]++
	if s.fail != nil {
		return nil, s.fail
	}
	return s.Memory.Get(service, account)
}

func (s *countingStore) reads() (n int) {
	for _, c := range s.gets {
		n += c
	}
	return n
}

func TestVaultOneSystemReadPerRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.vault")
	store := newCounting()
	// Saved before the vault, as their own items.
	store.Memory.Set(ServiceSnapshotKey, "app", []byte("snap"))
	store.Memory.Set(ServiceProxy, "p1", []byte("pw1"))

	v := NewVault(path, store)
	if err := v.Unlock(); err != nil {
		t.Fatal(err)
	}
	if err := v.Set(ServiceAPIToken, "app", []byte("tok")); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if s, err := v.Get(ServiceSnapshotKey, "app"); err != nil || string(s) != "snap" {
			t.Fatalf("snapshot key = %q, %v", s, err)
		}
	}
	if s, _ := v.Get(ServiceProxy, "p1"); string(s) != "pw1" {
		t.Fatalf("proxy = %q", s)
	}

	// The next run (a new build): only the vault key comes from the store.
	store.gets = map[string]int{}
	v2 := NewVault(path, store)
	for _, c := range []struct{ svc, acct, want string }{
		{ServiceAPIToken, "app", "tok"}, {ServiceSnapshotKey, "app", "snap"}, {ServiceProxy, "p1", "pw1"},
	} {
		if s, err := v2.Get(c.svc, c.acct); err != nil || string(s) != c.want {
			t.Fatalf("%s = %q, %v", c.svc, s, err)
		}
	}
	if store.reads() != 1 || store.gets[ServiceVaultKey] != 1 {
		t.Fatalf("system store read %v, want the vault key once", store.gets)
	}

	// Nothing readable in the file without the key.
	raw, _ := os.ReadFile(path)
	for _, s := range []string{"snap", "pw1", "tok"} {
		if bytes.Contains(raw, []byte(s)) {
			t.Fatalf("file shows %q in the clear", s)
		}
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Fatalf("file mode %v", fi.Mode().Perm())
	}
}

func TestVaultDeleteKeepsTheLegacyCopyAway(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.vault")
	store := newCounting()
	store.Memory.Set(ServiceProxy, "p1", []byte("pw1"))
	v := NewVault(path, store)
	if err := v.Delete(ServiceProxy, "p1"); err != nil {
		t.Fatal(err)
	}
	if _, err := NewVault(path, store).Get(ServiceProxy, "p1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted proxy password came back: %v", err)
	}
	if err := v.Set(ServiceProxy, "p1", []byte("new")); err != nil {
		t.Fatal(err)
	}
	if s, _ := NewVault(path, store).Get(ServiceProxy, "p1"); string(s) != "new" {
		t.Fatalf("after Set = %q", s)
	}
}

func TestVaultCancelledThenAllowed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.vault")
	store := newCounting()
	NewVault(path, store).Set(ServiceAPIToken, "app", []byte("tok"))
	store.fail = errors.New("User canceled the operation")
	v := NewVault(path, store)
	if _, err := v.Get(ServiceAPIToken, "app"); err == nil {
		t.Fatal("want the store's error")
	}
	store.fail = nil
	if s, err := v.Get(ServiceAPIToken, "app"); err != nil || string(s) != "tok" {
		t.Fatalf("retry = %q, %v", s, err)
	}
}

func TestVaultRefusesALostKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.vault")
	store := newCounting()
	NewVault(path, store).Set(ServiceAPIToken, "app", []byte("tok"))
	store.Memory.Delete(ServiceVaultKey, "app")
	if _, err := NewVault(path, store).Get(ServiceAPIToken, "app"); !errors.Is(err, ErrVaultKeyLost) {
		t.Fatalf("err = %v", err)
	}
	if _, err := store.Memory.Get(ServiceVaultKey, "app"); !errors.Is(err, ErrNotFound) {
		t.Fatal("a new key was made over the old file")
	}
}

func TestVaultCopiesAreTheCallers(t *testing.T) {
	v := NewVault(filepath.Join(t.TempDir(), "secrets.vault"), newCounting())
	v.Set(ServiceCookieKey, "pr", []byte("key"))
	s, _ := v.Get(ServiceCookieKey, "pr")
	Wipe(s) // the launcher wipes the key after handing it over
	if s, _ := v.Get(ServiceCookieKey, "pr"); string(s) != "key" {
		t.Fatalf("wiping a copy changed the vault: %q", s)
	}
}
