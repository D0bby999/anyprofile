// SPDX-License-Identifier: AGPL-3.0-or-later

// Package secrets keeps proxy passwords and per-profile cookie keys out of the
// database, logs and command lines. Production storage is the macOS Keychain.
package secrets

import (
	"errors"
	"sync"
)

// Keychain services. The account is the proxy or profile id.
const (
	ServiceProxy           = "dev.anyprofile.proxy"
	ServiceCookieKey       = "dev.anyprofile.cookie-key"
	ServiceAccountPassword = "dev.anyprofile.account-password"
	ServiceTOTP            = "dev.anyprofile.totp"
	// ServiceSnapshotKey holds the one key that encrypts every snapshot; account "app".
	ServiceSnapshotKey = "dev.anyprofile.snapshot-key"
)

var ErrNotFound = errors.New("secret not found")

// Store reads and writes opaque secrets addressed by service and account.
type Store interface {
	Get(service, account string) ([]byte, error)
	Set(service, account string, data []byte) error
	Delete(service, account string) error // deleting a missing item is not an error
}

// Memory is an in-process Store for tests.
type Memory struct {
	mu sync.Mutex
	m  map[string][]byte
}

func NewMemory() *Memory { return &Memory{m: map[string][]byte{}} }

func (s *Memory) Get(service, account string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[service+"\x00"+account]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), v...), nil
}

func (s *Memory) Set(service, account string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[service+"\x00"+account] = append([]byte(nil), data...)
	return nil
}

func (s *Memory) Delete(service, account string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, service+"\x00"+account)
	return nil
}

// Wipe zeroes a secret buffer once it has been handed off.
func Wipe(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
