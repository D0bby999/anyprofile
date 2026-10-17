// SPDX-License-Identifier: AGPL-3.0-or-later

package secrets

import (
	"errors"

	"github.com/keybase/go-keychain"
)

// Keychain stores secrets as generic passwords in the user's login keychain
// through the Security framework (no /usr/bin/security subprocess, so a secret
// never appears in a process listing). Items are never synchronizable.
type Keychain struct{}

func (Keychain) Get(service, account string) ([]byte, error) {
	q := keychain.NewItem()
	q.SetSecClass(keychain.SecClassGenericPassword)
	q.SetService(service)
	q.SetAccount(account)
	q.SetMatchLimit(keychain.MatchLimitOne)
	q.SetReturnData(true)
	res, err := keychain.QueryItem(q)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, ErrNotFound
	}
	return res[0].Data, nil
}

func (Keychain) Set(service, account string, data []byte) error {
	item := keychain.NewGenericPassword(service, account, "AnyProfile", data, "")
	item.SetSynchronizable(keychain.SynchronizableNo)
	item.SetAccessible(keychain.AccessibleWhenUnlockedThisDeviceOnly)
	err := keychain.AddItem(item)
	if !errors.Is(err, keychain.ErrorDuplicateItem) {
		return err
	}
	q := keychain.NewItem()
	q.SetSecClass(keychain.SecClassGenericPassword)
	q.SetService(service)
	q.SetAccount(account)
	upd := keychain.NewItem()
	upd.SetData(data)
	return keychain.UpdateItem(q, upd)
}

// Accounts lists the accounts stored under service. Only names are read, so
// macOS does not ask for access.
func (Keychain) Accounts(service string) ([]string, error) {
	accts, err := keychain.GetGenericPasswordAccounts(service)
	if errors.Is(err, keychain.ErrorItemNotFound) {
		return nil, nil
	}
	return accts, err
}

func (Keychain) Delete(service, account string) error {
	err := keychain.DeleteGenericPasswordItem(service, account)
	if errors.Is(err, keychain.ErrorItemNotFound) {
		return nil
	}
	return err
}
