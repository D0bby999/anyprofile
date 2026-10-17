// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build keychain

package secrets

import (
	"slices"
	"testing"
)

// Accounts lists by name, which Vault.MoveIn relies on to find legacy items.
func TestKeychainAccounts(t *testing.T) {
	const svc = "dev.anyprofile.test-accounts"
	k := Keychain{}
	if accts, err := k.Accounts(svc); err != nil || len(accts) != 0 {
		t.Fatalf("empty service = %v, %v", accts, err)
	}
	for _, a := range []string{"one", "two"} {
		t.Cleanup(func() { _ = k.Delete(svc, a) })
		if err := k.Set(svc, a, []byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	accts, err := k.Accounts(svc)
	slices.Sort(accts)
	if err != nil || !slices.Equal(accts, []string{"one", "two"}) {
		t.Fatalf("accounts = %v, %v", accts, err)
	}
}
