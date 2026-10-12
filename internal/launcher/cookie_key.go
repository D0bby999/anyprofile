// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/D0bby999/anyprofile/internal/secrets"
)

// The core encrypts cookies with a 32-byte key handed over on a pipe. A wrong
// key silently discards the whole cookie jar (Fury patch 0110), so a
// fingerprint of the key is kept beside the profile and a mismatch refuses
// the launch instead of logging the user out of every account.
const keyFingerprintFile = ".anyprofile-key-fingerprint"

var ErrCookieKeyLost = errors.New("the cookie key for this profile is missing from the Keychain; " +
	"opening it with a new key would discard its saved logins")

func keyFingerprint(key []byte) string {
	sum := sha256.Sum256(append([]byte("anyprofile-cookie-key-v1\x00"), key...))
	return hex.EncodeToString(sum[:16])
}

// cookieKey returns the profile's key, creating it on first launch. The
// caller wipes the returned slice after handing it to the browser.
func cookieKey(sec secrets.Store, profileID, userDataDir string) ([]byte, error) {
	fpPath := filepath.Join(userDataDir, keyFingerprintFile)
	wantFP, fpErr := os.ReadFile(fpPath)
	key, err := sec.Get(secrets.ServiceCookieKey, profileID)
	switch {
	case err == nil:
		if len(key) != 32 {
			return nil, fmt.Errorf("cookie key in Keychain has %d bytes, want 32", len(key))
		}
		if fpErr == nil && !bytes.Equal(bytes.TrimSpace(wantFP), []byte(keyFingerprint(key))) {
			return nil, errors.New("the Keychain holds a different cookie key than this profile was created with")
		}
		if fpErr != nil {
			if err := os.WriteFile(fpPath, []byte(keyFingerprint(key)), 0o600); err != nil {
				return nil, err
			}
		}
		return key, nil
	case errors.Is(err, secrets.ErrNotFound):
		if fpErr == nil {
			return nil, ErrCookieKeyLost
		}
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		if err := sec.Set(secrets.ServiceCookieKey, profileID, key); err != nil {
			return nil, err
		}
		if err := os.WriteFile(fpPath, []byte(keyFingerprint(key)), 0o600); err != nil {
			return nil, err
		}
		return key, nil
	default:
		return nil, fmt.Errorf("read cookie key: %w", err)
	}
}
