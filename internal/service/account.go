// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"errors"
	"slices"
	"strings"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
	"github.com/D0bby999/anyprofile/internal/totp"
)

// TOTPCode is the code valid right now and how long it stays valid.
type TOTPCode struct {
	Code        string `json:"code"`
	SecondsLeft int    `json:"secondsLeft"`
}

// account loads the profile's account on site. A new account may only be
// made for one of the profile's platforms; an existing one (its platform
// since removed) can still be read, changed and forgotten.
func (s *Service) account(id, site string) (store.Account, error) {
	p, err := s.Store.GetProfile(id)
	if err != nil {
		return store.Account{}, err
	}
	a, err := s.Store.Account(id, site)
	if err != nil {
		return a, err
	}
	known := slices.ContainsFunc(p.Accounts, func(x store.Account) bool { return x.Site == site })
	if !known && !slices.Contains(p.Sites, site) {
		return a, apperr.New("account_site", "the profile is not on that platform")
	}
	return a, nil
}

// SetAccountLogin saves the login (email or username) of one platform.
func (s *Service) SetAccountLogin(id, site, login string) error {
	a, err := s.account(id, site)
	if err != nil {
		return err
	}
	a.Login = strings.TrimSpace(login)
	if err := s.Store.SaveAccount(id, a); err != nil {
		return err
	}
	s.notify("profiles")
	return nil
}

// SetAccountPassword stores the password in the Keychain; only the flag goes
// into SQLite. The password never reaches the frontend.
func (s *Service) SetAccountPassword(id, site, pw string) error {
	if pw == "" {
		return apperr.New("password_empty", "type a password")
	}
	a, err := s.account(id, site)
	if err != nil {
		return err
	}
	if err := s.Secrets.Set(secrets.ServiceAccountPassword, a.SecretKey, []byte(pw)); err != nil {
		return err
	}
	a.HasPassword = true
	return s.saveAccount(id, a)
}

// ClearAccountPassword removes the stored password.
func (s *Service) ClearAccountPassword(id, site string) error {
	a, err := s.account(id, site)
	if err != nil {
		return err
	}
	if err := s.Secrets.Delete(secrets.ServiceAccountPassword, a.SecretKey); err != nil {
		return err
	}
	a.HasPassword = false
	return s.saveAccount(id, a)
}

// AccountPassword reads the password for the clipboard binding only. It is
// never bound to the frontend.
func (s *Service) AccountPassword(id, site string) ([]byte, error) {
	a, err := s.Store.Account(id, site)
	if err != nil {
		return nil, err
	}
	pw, err := s.Secrets.Get(secrets.ServiceAccountPassword, a.SecretKey)
	if !a.HasPassword || errors.Is(err, secrets.ErrNotFound) {
		return nil, apperr.New("password_missing", "no password is saved for this account")
	}
	return pw, err
}

// SetTOTP parses an authenticator secret and stores the raw key in the Keychain.
func (s *Service) SetTOTP(id, site, secret string) error {
	key, err := totp.ParseSecret(secret)
	if err != nil {
		return apperr.New("totp_invalid", err.Error())
	}
	defer secrets.Wipe(key)
	a, err := s.account(id, site)
	if err != nil {
		return err
	}
	if err := s.Secrets.Set(secrets.ServiceTOTP, a.SecretKey, key); err != nil {
		return err
	}
	a.HasTOTP = true
	return s.saveAccount(id, a)
}

// ClearTOTP removes the stored 2FA secret.
func (s *Service) ClearTOTP(id, site string) error {
	a, err := s.account(id, site)
	if err != nil {
		return err
	}
	if err := s.Secrets.Delete(secrets.ServiceTOTP, a.SecretKey); err != nil {
		return err
	}
	a.HasTOTP = false
	return s.saveAccount(id, a)
}

// TOTPCode returns the code valid right now. The secret never leaves Go.
func (s *Service) TOTPCode(id, site string) (TOTPCode, error) {
	a, err := s.Store.Account(id, site)
	if err != nil {
		return TOTPCode{}, err
	}
	key, err := s.Secrets.Get(secrets.ServiceTOTP, a.SecretKey)
	if !a.HasTOTP || errors.Is(err, secrets.ErrNotFound) {
		return TOTPCode{}, apperr.New("totp_missing", "no 2FA secret is saved for this account")
	}
	if err != nil {
		return TOTPCode{}, err
	}
	code, left := totp.Code(key, s.now())
	secrets.Wipe(key)
	return TOTPCode{Code: code, SecondsLeft: left}, nil
}

// ForgetAccount removes an account and its Keychain items.
func (s *Service) ForgetAccount(id, site string) error {
	a, err := s.Store.Account(id, site)
	if err != nil {
		return err
	}
	if err := s.deleteAccountSecrets(a.SecretKey); err != nil {
		return err
	}
	if err := s.Store.DeleteAccount(id, site); err != nil {
		return err
	}
	s.notify("profiles")
	return nil
}

func (s *Service) saveAccount(id string, a store.Account) error {
	if err := s.Store.SaveAccount(id, a); err != nil {
		return err
	}
	s.notify("profiles")
	return nil
}

func (s *Service) deleteAccountSecrets(key string) error {
	if err := s.Secrets.Delete(secrets.ServiceAccountPassword, key); err != nil {
		return err
	}
	return s.Secrets.Delete(secrets.ServiceTOTP, key)
}
