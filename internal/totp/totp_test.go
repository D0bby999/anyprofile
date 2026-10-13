// SPDX-License-Identifier: AGPL-3.0-or-later

package totp

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

var key = []byte("12345678901234567890")

func TestCodeRFC6238Vectors(t *testing.T) {
	for _, v := range []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
		{20000000000, "353130"},
	} {
		got, _ := Code(key, time.Unix(v.unix, 0))
		if got != v.want {
			t.Errorf("Code(%d) = %s, want %s", v.unix, got, v.want)
		}
	}
	if _, left := Code(key, time.Unix(59, 0)); left != 1 {
		t.Errorf("seconds left at t=59 = %d, want 1", left)
	}
}

func TestParseSecret(t *testing.T) {
	if got, err := ParseSecret("gezd gnbv gy3t qojq gezd gnbv gy3t qojq"); err != nil || !bytes.Equal(got, key) {
		t.Errorf("base32 with spaces = %v, %v", got, err)
	}
	if got, err := ParseSecret("otpauth://totp/Etsy:me?secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ&issuer=Etsy"); err != nil || !bytes.Equal(got, key) {
		t.Errorf("otpauth link = %v, %v", got, err)
	}
	if _, err := ParseSecret("otpauth://totp/x?secret=GEZDGNBVGY3TQOJQ&digits=8"); !errors.Is(err, ErrInvalid) {
		t.Errorf("digits=8 err = %v, want ErrInvalid", err)
	}
	if _, err := ParseSecret("hello!"); err == nil {
		t.Error("hello! must fail")
	}
	if _, err := ParseSecret("GEZDGNBV"); err == nil {
		t.Error("a short secret must fail")
	}
}
