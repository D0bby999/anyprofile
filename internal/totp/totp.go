// SPDX-License-Identifier: AGPL-3.0-or-later

// Package totp computes RFC 6238 codes (HMAC-SHA1, 6 digits, 30 s), the
// kind every major platform's authenticator setup uses.
package totp

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const Period = 30

var ErrInvalid = errors.New("not a valid authenticator secret")

// ParseSecret accepts a base32 secret (spaces, dashes, lowercase and padding
// allowed) or an otpauth://totp/ link, and returns the raw key.
func ParseSecret(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(strings.ToLower(s), "otpauth://") {
		u, err := url.Parse(s)
		if err != nil || !strings.EqualFold(u.Host, "totp") {
			return nil, ErrInvalid
		}
		q := u.Query()
		if a := q.Get("algorithm"); a != "" && !strings.EqualFold(a, "SHA1") {
			return nil, fmt.Errorf("%w: only SHA1 codes are supported", ErrInvalid)
		}
		if d := q.Get("digits"); d != "" && d != "6" {
			return nil, fmt.Errorf("%w: only 6-digit codes are supported", ErrInvalid)
		}
		if p := q.Get("period"); p != "" && p != "30" {
			return nil, fmt.Errorf("%w: only 30-second codes are supported", ErrInvalid)
		}
		s = q.Get("secret")
	}
	s = strings.ToUpper(strings.NewReplacer(" ", "", "-", "", "=", "").Replace(s))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
	if err != nil || len(key) < 10 {
		return nil, ErrInvalid
	}
	return key, nil
}

// Code returns the 6-digit code at t and how many seconds it stays valid.
func Code(key []byte, t time.Time) (string, int) {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(t.Unix())/Period)
	m := hmac.New(sha1.New, key)
	m.Write(msg[:])
	h := m.Sum(nil)
	off := h[len(h)-1] & 0x0f
	bin := binary.BigEndian.Uint32(h[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", bin%1_000_000), Period - int(t.Unix()%Period)
}
