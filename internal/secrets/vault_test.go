// SPDX-License-Identifier: AGPL-3.0-or-later

package secrets

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
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
	// Windows has no Unix modes: the file is the user's by its folder's ACL.
	if fi, _ := os.Stat(path); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
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

// MoveIn asks for every legacy item at launch, then they are gone from the
// system store: nothing is asked later (a profile closing reads the
// snapshot key), and no old password is left behind.
func TestVaultMoveIn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.vault")
	store := newCounting()
	legacy := map[[2]string]string{
		{ServiceAPIToken, "app"}: "tok", {ServiceSnapshotKey, "app"}: "snap",
		{ServiceProxy, "p1"}: "pw1", {ServiceProxy, "p2"}: "pw2",
		{ServiceCookieKey, "pr1"}: "ck1", {ServiceAccountPassword, "a1"}: "apw", {ServiceTOTP, "a1"}: "otp",
	}
	for k, s := range legacy {
		store.Memory.Set(k[0], k[1], []byte(s))
	}
	v := NewVault(path, store)
	// Already moved in on first use by the build before: only removed.
	if s, _ := v.Get(ServiceProxy, "p2"); string(s) != "pw2" {
		t.Fatalf("p2 = %q", s)
	}
	store.fail = errors.New("User canceled the operation")
	if err := v.MoveIn(); err == nil {
		t.Fatal("a cancelled answer must stop MoveIn")
	}
	store.fail = nil
	if err := v.MoveIn(); err != nil {
		t.Fatal(err)
	}
	for k := range legacy {
		if _, err := store.Memory.Get(k[0], k[1]); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%v is still a system item", k)
		}
	}
	if _, err := store.Memory.Get(ServiceVaultKey, "app"); err != nil {
		t.Fatal("the vault key went too")
	}
	store.gets = map[string]int{}
	v2 := NewVault(path, store)
	if err := v2.MoveIn(); err != nil {
		t.Fatal(err)
	}
	for k, want := range legacy {
		if s, err := v2.Get(k[0], k[1]); err != nil || string(s) != want {
			t.Fatalf("%v = %q, %v", k, s, err)
		}
	}
	if store.reads() != 1 {
		t.Fatalf("next launch read %v, want the vault key once", store.gets)
	}
}

// A store that cannot list (none today) leaves legacy items to first use.
func TestVaultMoveInWithoutAList(t *testing.T) {
	store := struct{ Store }{NewMemory()}
	store.Set(ServiceProxy, "p1", []byte("pw1"))
	v := NewVault(filepath.Join(t.TempDir(), "secrets.vault"), store)
	if err := v.MoveIn(); err != nil {
		t.Fatal(err)
	}
	if s, _ := v.Get(ServiceProxy, "p1"); string(s) != "pw1" {
		t.Fatalf("p1 = %q", s)
	}
}

func TestVaultDeleteRemovesTheLegacyItem(t *testing.T) {
	store := newCounting()
	store.Memory.Set(ServiceProxy, "p1", []byte("pw1"))
	v := NewVault(filepath.Join(t.TempDir(), "secrets.vault"), store)
	if err := v.Delete(ServiceProxy, "p1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Memory.Get(ServiceProxy, "p1"); !errors.Is(err, ErrNotFound) {
		t.Fatal("the old password is still in the system store")
	}
}
