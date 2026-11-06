// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"database/sql"
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/crypto/pbkdf2"

	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

// serviceChromiumSafeStorage holds a backup's macOS "Chromium Safe Storage"
// Keychain password until an imported AnyBrowser profile's cookie jar has been
// re-encrypted to this computer's key.
const serviceChromiumSafeStorage = "dev.anyprofile.chromium-safe-storage"

var (
	chromiumV10Prefix = []byte("v10")
	chromiumSalt      = []byte("saltysalt")
	chromiumSpaceIV   = bytes.Repeat([]byte{' '}, aes.BlockSize)
)

// hasBrowserCookies reports whether any profile that manages its own cookie
// key (AnyBrowser) has a cookie jar on disk.
func (s *Service) hasBrowserCookies(profiles []store.Profile) bool {
	for _, p := range profiles {
		if eng := s.Engines[p.Engine]; eng != nil && eng.UsesCookieKey() {
			continue
		}
		if fileExists(filepath.Join(s.Dirs.Profile(p.ID), "Default", "Cookies")) {
			return true
		}
	}
	return false
}

// reencryptProfileCookies re-encrypts an imported profile's Chromium cookie jar
// from the source Mac's "Chromium Safe Storage" password to this Mac's
// password, then forgets the source password.
func (s *Service) reencryptProfileCookies(profileID string) {
	if s.BrowserCookieKey == nil || s.Secrets == nil {
		return
	}
	srcPass, err := s.Secrets.Get(serviceChromiumSafeStorage, profileID)
	if err != nil || len(srcPass) == 0 {
		return
	}
	defer secrets.Wipe(srcPass)
	dstPass, err := s.BrowserCookieKey()
	if err != nil || len(dstPass) == 0 {
		return
	}
	defer secrets.Wipe(dstPass)
	if !bytes.Equal(srcPass, dstPass) {
		jar := filepath.Join(s.Dirs.Profile(profileID), "Default", "Cookies")
		if err := reencryptChromiumCookies(jar, srcPass, dstPass); err != nil {
			if s.Log != nil {
				s.Log.Warn("re-encrypt browser cookies", "profile", profileID, "err", err)
			}
			return
		}
	}
	_ = s.Secrets.Delete(serviceChromiumSafeStorage, profileID)
}

func deriveChromiumMacKey(password []byte) []byte {
	return pbkdf2.Key(password, chromiumSalt, 1003, 16, sha1.New)
}

func decryptChromiumV10(block cipher.Block, enc []byte) ([]byte, bool) {
	if !bytes.HasPrefix(enc, chromiumV10Prefix) {
		return nil, false
	}
	ct := enc[len(chromiumV10Prefix):]
	if len(ct) == 0 || len(ct)%aes.BlockSize != 0 {
		return nil, false
	}
	plain := make([]byte, len(ct))
	cipher.NewCBCDecrypter(block, chromiumSpaceIV).CryptBlocks(plain, ct)
	pad := int(plain[len(plain)-1])
	if pad < 1 || pad > aes.BlockSize || pad > len(plain) {
		return nil, false
	}
	for _, b := range plain[len(plain)-pad:] {
		if int(b) != pad {
			return nil, false
		}
	}
	return plain[:len(plain)-pad], true
}

func encryptChromiumV10(block cipher.Block, plain []byte) []byte {
	pad := aes.BlockSize - (len(plain) % aes.BlockSize)
	buf := make([]byte, len(chromiumV10Prefix)+len(plain)+pad)
	copy(buf, chromiumV10Prefix)
	body := buf[len(chromiumV10Prefix):]
	copy(body, plain)
	for i := len(plain); i < len(body); i++ {
		body[i] = byte(pad)
	}
	cipher.NewCBCEncrypter(block, chromiumSpaceIV).CryptBlocks(body, body)
	return buf
}

// reencryptChromiumCookies rewrites v10-encrypted cookie rows in a Chromium
// Cookies SQLite database from srcPass to dstPass.
func reencryptChromiumCookies(dbPath string, srcPass, dstPass []byte) error {
	if _, err := os.Stat(dbPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	srcKey := deriveChromiumMacKey(srcPass)
	defer secrets.Wipe(srcKey)
	dstKey := deriveChromiumMacKey(dstPass)
	defer secrets.Wipe(dstKey)
	srcBlock, err := aes.NewCipher(srcKey)
	if err != nil {
		return err
	}
	dstBlock, err := aes.NewCipher(dstKey)
	if err != nil {
		return err
	}

	db, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=busy_timeout(2000)")
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`SELECT rowid, encrypted_value FROM cookies WHERE length(encrypted_value) > 3`)
	if err != nil {
		return err
	}
	type cookieUpdate struct {
		rowid int64
		enc   []byte
	}
	var updates []cookieUpdate
	for rows.Next() {
		var id int64
		var enc []byte
		if err := rows.Scan(&id, &enc); err != nil {
			rows.Close()
			return err
		}
		plain, ok := decryptChromiumV10(srcBlock, enc)
		if !ok {
			continue
		}
		reenc := encryptChromiumV10(dstBlock, plain)
		secrets.Wipe(plain)
		updates = append(updates, cookieUpdate{rowid: id, enc: reenc})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, u := range updates {
		if _, err := tx.Exec(`UPDATE cookies SET encrypted_value = ? WHERE rowid = ?`, u.enc, u.rowid); err != nil {
			return err
		}
	}
	return tx.Commit()
}
