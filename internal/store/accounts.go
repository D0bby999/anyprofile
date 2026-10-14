// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// Account is a profile's login on one platform. The password and the 2FA
// secret live in the Keychain under SecretKey; only flags are stored here.
type Account struct {
	Site        string `json:"site"`
	Login       string `json:"login"`
	HasPassword bool   `json:"hasPassword"`
	HasTOTP     bool   `json:"hasTotp"`
	SecretKey   string `json:"-"`
}

const accountCols = `site, login, has_password, has_totp, secret_key`

func scanAccount(rows *sql.Rows) (string, Account, error) {
	var id string
	var a Account
	err := rows.Scan(&id, &a.Site, &a.Login, &a.HasPassword, &a.HasTOTP, &a.SecretKey)
	return id, a, err
}

// accountsByProfile loads the accounts of one profile, or of all when id is "".
func (s *Store) accountsByProfile(id string) (map[string][]Account, error) {
	q := `SELECT profile_id, ` + accountCols + ` FROM accounts`
	var args []any
	if id != "" {
		q += ` WHERE profile_id = ?`
		args = append(args, id)
	}
	rows, err := s.db.Query(q+` ORDER BY rowid`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]Account{}
	for rows.Next() {
		pid, a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		out[pid] = append(out[pid], a)
	}
	return out, rows.Err()
}

// Account returns the profile's account on site; a new one (not yet saved)
// when there is none, with the Keychain key it will use.
func (s *Store) Account(profileID, site string) (Account, error) {
	a := Account{Site: site, SecretKey: profileID + "/" + site}
	err := s.db.QueryRow(`SELECT `+accountCols+` FROM accounts WHERE profile_id = ? AND site = ?`, profileID, site).
		Scan(&a.Site, &a.Login, &a.HasPassword, &a.HasTOTP, &a.SecretKey)
	if errors.Is(err, sql.ErrNoRows) {
		return a, nil
	}
	return a, err
}

// SaveAccount writes the account, or deletes it once it holds nothing.
func (s *Store) SaveAccount(profileID string, a Account) error {
	if a.Site == "" || a.SecretKey == "" {
		return fmt.Errorf("account needs a site and a secret key")
	}
	if a.Login == "" && !a.HasPassword && !a.HasTOTP {
		return s.DeleteAccount(profileID, a.Site)
	}
	_, err := s.db.Exec(`INSERT INTO accounts (profile_id, `+accountCols+`) VALUES (?,?,?,?,?,?)
		ON CONFLICT (profile_id, site) DO UPDATE SET login = excluded.login,
		has_password = excluded.has_password, has_totp = excluded.has_totp`,
		profileID, a.Site, a.Login, a.HasPassword, a.HasTOTP, a.SecretKey)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE profiles SET updated_at = ? WHERE id = ?`, now(), profileID)
	return err
}

// DeleteAccount forgets the account row; the caller removes its Keychain items.
func (s *Store) DeleteAccount(profileID, site string) error {
	_, err := s.db.Exec(`DELETE FROM accounts WHERE profile_id = ? AND site = ?`, profileID, site)
	return err
}
