// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"errors"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/totp"
)

// TOTPCode is the code valid right now and how long it stays valid.
type TOTPCode struct {
	Code        string `json:"code"`
	SecondsLeft int    `json:"secondsLeft"`
}

// SetAccountPassword stores the password in the Keychain; only the flag goes
// into SQLite. The password never reaches the frontend.
func (s *Service) SetAccountPassword(id, pw string) error {
	if pw == "" {
		return apperr.New("password_empty", "type a password")
	}
	if _, err := s.Store.GetProfile(id); err != nil {
		return err
	}
	if err := s.Secrets.Set(secrets.ServiceAccountPassword, id, []byte(pw)); err != nil {
		return err
	}
	if err := s.Store.SetSecretFlag(id, "has_password", true); err != nil {
		return err
	}
	s.notify("profiles")
	return nil
}

// ClearAccountPassword removes the stored password.
func (s *Service) ClearAccountPassword(id string) error {
	if err := s.Secrets.Delete(secrets.ServiceAccountPassword, id); err != nil {
		return err
	}
	if err := s.Store.SetSecretFlag(id, "has_password", false); err != nil {
		return err
	}
	s.notify("profiles")
	return nil
}

// AccountPassword reads the password for the clipboard binding only. It is
// never bound to the frontend.
func (s *Service) AccountPassword(id string) ([]byte, error) {
	pw, err := s.Secrets.Get(secrets.ServiceAccountPassword, id)
	if errors.Is(err, secrets.ErrNotFound) {
		return nil, apperr.New("password_missing", "no password is saved for this profile")
	}
	return pw, err
}

// SetTOTP parses an authenticator secret and stores the raw key in the Keychain.
func (s *Service) SetTOTP(id, secret string) error {
	key, err := totp.ParseSecret(secret)
	if err != nil {
		return apperr.New("totp_invalid", err.Error())
	}
	if _, err := s.Store.GetProfile(id); err != nil {
		return err
	}
	if err := s.Secrets.Set(secrets.ServiceTOTP, id, key); err != nil {
		return err
	}
	secrets.Wipe(key)
	if err := s.Store.SetSecretFlag(id, "has_totp", true); err != nil {
		return err
	}
	s.notify("profiles")
	return nil
}

// ClearTOTP removes the stored 2FA secret.
func (s *Service) ClearTOTP(id string) error {
	if err := s.Secrets.Delete(secrets.ServiceTOTP, id); err != nil {
		return err
	}
	if err := s.Store.SetSecretFlag(id, "has_totp", false); err != nil {
		return err
	}
	s.notify("profiles")
	return nil
}

// TOTPCode returns the code valid right now. The secret never leaves Go.
func (s *Service) TOTPCode(id string) (TOTPCode, error) {
	key, err := s.Secrets.Get(secrets.ServiceTOTP, id)
	if errors.Is(err, secrets.ErrNotFound) {
		return TOTPCode{}, apperr.New("totp_missing", "no 2FA secret is saved for this profile")
	}
	if err != nil {
		return TOTPCode{}, err
	}
	code, left := totp.Code(key, s.now())
	secrets.Wipe(key)
	return TOTPCode{Code: code, SecondsLeft: left}, nil
}
