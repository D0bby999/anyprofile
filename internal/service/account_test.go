// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

func TestAccount(t *testing.T) {
	s, sec := newService(t)
	ctx := context.Background()
	s.Clock = func() time.Time { return time.Unix(59, 0) }

	p, err := s.CreateProfile(ctx, NewProfile{Name: "Vault", Sites: []string{"tiktok", "gmail"}})
	if err != nil {
		t.Fatal(err)
	}
	id := p.ID

	if err := s.SetTOTP(id, "tiktok", "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"); err != nil {
		t.Fatal(err)
	}
	code, err := s.TOTPCode(id, "tiktok")
	if err != nil {
		t.Fatal(err)
	}
	if code.Code != "287082" || code.SecondsLeft != 1 {
		t.Fatalf("code = %+v", code)
	}
	if err := s.SetTOTP(id, "tiktok", "nope"); err == nil || !strings.Contains(err.Error(), "totp_invalid") {
		t.Fatalf("bad secret err = %v", err)
	}

	if err := s.SetAccountPassword(id, "gmail", "S3cret-pass!"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAccountPassword(id, "gmail", ""); err == nil || !strings.Contains(err.Error(), "password_empty") {
		t.Fatalf("empty password err = %v", err)
	}
	pw, err := s.AccountPassword(id, "gmail")
	if err != nil || string(pw) != "S3cret-pass!" {
		t.Fatalf("password = %q, %v", pw, err)
	}
	// Each platform has its own account: TikTok has no password, Gmail no 2FA.
	if _, err := s.AccountPassword(id, "tiktok"); err == nil || !strings.Contains(err.Error(), "password_missing") {
		t.Fatalf("tiktok password err = %v", err)
	}
	if _, err := s.TOTPCode(id, "gmail"); err == nil || !strings.Contains(err.Error(), "totp_missing") {
		t.Fatalf("gmail code err = %v", err)
	}
	if err := s.SetAccountLogin(id, "tiktok", "  @vault  "); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAccountLogin(id, "etsy", "x"); err == nil || !strings.Contains(err.Error(), "account_site") {
		t.Fatalf("account on a platform the profile is not on: %v", err)
	}
	got, _ := s.Store.GetProfile(id)
	if len(got.Accounts) != 2 || got.Accounts[0].Login != "@vault" || !got.Accounts[0].HasTOTP || !got.Accounts[1].HasPassword {
		t.Fatalf("accounts = %+v", got.Accounts)
	}
	// Removing a platform keeps its account until it is forgotten.
	if err := s.SetSitesMany([]string{id}, []string{"tiktok"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAccountLogin(id, "gmail", "kept@example.com"); err != nil {
		t.Fatalf("account of a removed platform: %v", err)
	}
	if err := s.ForgetAccount(id, "gmail"); err != nil {
		t.Fatal(err)
	}
	if _, err := sec.Get(secrets.ServiceAccountPassword, id+"/gmail"); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("forgotten password kept: %v", err)
	}
	if err := s.SetAccountPassword(id, "tiktok", "S3cret-pass!"); err != nil {
		t.Fatal(err)
	}

	// The password never reaches the frontend.
	rows, err := s.ListProfiles(store.ProfileFilter{})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(rows)
	if bytes.Contains(b, []byte("S3cret-pass!")) {
		t.Fatal("password leaked into ListProfiles JSON")
	}
	if !bytes.Contains(b, []byte(`"hasPassword":true`)) {
		t.Fatalf("hasPassword flag missing: %s", b)
	}

	// Purging deletes every secret of the profile.
	if err := s.Trash(id); err != nil {
		t.Fatal(err)
	}
	if err := s.Purge(id); err != nil {
		t.Fatal(err)
	}
	for _, svc := range []string{secrets.ServiceAccountPassword, secrets.ServiceTOTP} {
		if _, err := sec.Get(svc, id+"/tiktok"); !errors.Is(err, secrets.ErrNotFound) {
			t.Fatalf("%s survived purge: %v", svc, err)
		}
	}
}

func TestSecretsNeverTouchTheDB(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	p, err := s.CreateProfile(ctx, NewProfile{Name: "Vault"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetAccountPassword(p.ID, "other", "S3cret-pass!"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTOTP(p.ID, "other", "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{s.Dirs.DB(), s.Dirs.DB() + "-wal"} {
		raw, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(raw, []byte("S3cret-pass!")) {
			t.Fatalf("password found in %s", path)
		}
		if bytes.Contains(raw, []byte("GEZDGNBVGY3TQOJQ")) {
			t.Fatalf("totp secret found in %s", path)
		}
	}
}
