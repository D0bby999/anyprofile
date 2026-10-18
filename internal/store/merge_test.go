// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestMergeAddsOnlyWhatIsMissing(t *testing.T) {
	ctx := context.Background()
	// The backup: two proxies, three profiles (one in the trash), accounts, an extension.
	src, _ := openTemp(t)
	for _, id := range []string{"pxA", "pxB"} {
		if err := src.CreateProxy(&Proxy{ID: id, Label: id, Scheme: "http", Host: "gw", Port: 1, Username: "u"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"shared", "new1", "new2"} {
		p := sample(id)
		p.ProxyID = "pxB"
		if err := src.CreateProfile(p); err != nil {
			t.Fatal(err)
		}
	}
	src.SoftDelete("new2")
	src.SaveAccount("new1", Account{Site: "youtube", Login: "me", HasPassword: true, SecretKey: "k1"})
	src.SaveAccount("shared", Account{Site: "tiktok", Login: "them", HasTOTP: true, SecretKey: "k2"})
	src.CreateExtension(&Extension{ID: "ext-1", Name: "E", Version: "1"})
	src.SetProfileExtension("new1", "ext-1", true)
	bak := filepath.Join(t.TempDir(), "bak.db")
	if err := src.CopyTo(bak); err != nil {
		t.Fatal(err)
	}

	// This computer already has pxA and "shared" (serial 1), and "mine" (serial 2).
	dst, _ := openTemp(t)
	dst.CreateProxy(&Proxy{ID: "pxA", Label: "mine", Scheme: "http", Host: "here", Port: 2, Username: "x"})
	here := sample("shared")
	here.Name = "Kept as it is"
	dst.CreateProfile(here)
	dst.CreateProfile(sample("mine"))

	var seen MergeResult
	res, err := dst.Merge(ctx, bak, func(r MergeResult) error { seen = r; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(seen.Profiles) != 2 || len(res.Proxies) != 1 || res.Proxies[0] != "pxB" || res.ProfilesHere != 1 || res.ProxiesHere != 1 {
		t.Fatalf("result = %+v", res)
	}
	if len(res.Extensions) != 1 || len(res.Accounts) != 1 || res.Accounts[0] != (MergedAccount{SecretKey: "k1", Password: true}) {
		t.Fatalf("extensions %v accounts %+v", res.Extensions, res.Accounts)
	}
	got, _ := dst.GetProfile("shared")
	if got.Name != "Kept as it is" {
		t.Fatalf("an existing profile was changed: %q", got.Name)
	}
	px, _ := dst.GetProxy("pxA")
	if px.Label != "mine" {
		t.Fatalf("an existing proxy was changed: %q", px.Label)
	}
	n1, _ := dst.GetProfile("new1")
	n2, _ := dst.GetProfile("new2")
	if n1.Serial != 3 || n2.Serial != 4 || n1.ProxyID != "pxB" || n2.DeletedAt == "" {
		t.Fatalf("new1 %+v / new2 serial %d deleted %q", n1.Serial, n2.Serial, n2.DeletedAt)
	}
	if a, err := dst.Account("new1", "youtube"); err != nil || a.Login != "me" {
		t.Fatalf("account = %+v, %v", a, err)
	}
	if a, _ := dst.Account("shared", "tiktok"); a.Login != "" || a.HasTOTP {
		t.Fatalf("an existing profile got an account: %+v", a)
	}

	// Again: everything is here already.
	res, err = dst.Merge(ctx, bak, nil)
	if err != nil || len(res.Profiles)+len(res.Proxies)+len(res.Extensions) != 0 || res.ProfilesHere != 3 {
		t.Fatalf("second merge = %+v, %v", res, err)
	}
}

func TestMergeRollsBack(t *testing.T) {
	ctx := context.Background()
	src, _ := openTemp(t)
	src.CreateProfile(sample("p1"))
	bak := filepath.Join(t.TempDir(), "bak.db")
	src.CopyTo(bak)
	dst, _ := openTemp(t)
	boom := errors.New("files did not move")
	if _, err := dst.Merge(ctx, bak, func(MergeResult) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if _, err := dst.GetProfile("p1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("profile added despite the error: %v", err)
	}
	// The database is usable afterwards (bak detached, connection back).
	if err := dst.CreateProfile(sample("p2")); err != nil {
		t.Fatal(err)
	}
}

func TestMergeRefusesANewerDatabase(t *testing.T) {
	src, _ := openTemp(t)
	src.db.Exec(`INSERT INTO schema_version (version) VALUES (999)`)
	bak := filepath.Join(t.TempDir(), "bak.db")
	src.CopyTo(bak)
	dst, _ := openTemp(t)
	if _, err := dst.Merge(context.Background(), bak, nil); !errors.Is(err, ErrNewerDatabase) {
		t.Fatalf("err = %v", err)
	}
}
