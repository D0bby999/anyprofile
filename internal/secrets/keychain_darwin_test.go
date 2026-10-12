// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build keychain

package secrets

import (
	"errors"
	"testing"
)

// Touches the real login keychain, so it only runs with -tags keychain.
func TestKeychainRoundTrip(t *testing.T) {
	const svc, acct = "dev.anyprofile.test", "roundtrip"
	k := Keychain{}
	t.Cleanup(func() { _ = k.Delete(svc, acct) })
	if err := k.Set(svc, acct, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := k.Set(svc, acct, []byte("two")); err != nil {
		t.Fatal(err)
	}
	got, err := k.Get(svc, acct)
	if err != nil || string(got) != "two" {
		t.Fatalf("get = %q, %v", got, err)
	}
	if err := k.Delete(svc, acct); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Get(svc, acct); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete err = %v", err)
	}
	if err := k.Delete(svc, acct); err != nil {
		t.Fatalf("second delete: %v", err)
	}
}
