// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"bytes"
	"context"
	"crypto/aes"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

const pw = "correct horse"

// A computer with a proxy, a profile that has files, a cookie key and an
// account with a password and 2FA, an extension and a recipe.
func backupSource(t *testing.T) (*Service, string) {
	s, sec := newService(t)
	ctx := context.Background()
	if err := s.Store.CreateProxy(&store.Proxy{ID: "px1", Label: "us", Scheme: "http", Host: "gw", Port: 1, Username: "u"}); err != nil {
		t.Fatal(err)
	}
	sec.Set(secrets.ServiceProxy, "px1", []byte("proxy-pw"))
	p, err := s.CreateProfile(ctx, NewProfile{Name: "Kênh A", Sites: []string{"other"}})
	if err != nil {
		t.Fatal(err)
	}
	p.ProxyID = "px1"
	if err := s.Store.UpdateProfile(p); err != nil {
		t.Fatal(err)
	}
	sec.Set(secrets.ServiceCookieKey, p.ID, []byte("cookie-key"))
	s.Store.SaveAccount(p.ID, store.Account{Site: "youtube", Login: "me", HasPassword: true, HasTOTP: true, SecretKey: "acct1"})
	sec.Set(secrets.ServiceAccountPassword, "acct1", []byte("site-pw"))
	sec.Set(secrets.ServiceTOTP, "acct1", []byte("totp"))
	dir := s.Dirs.Profile(p.ID)
	os.MkdirAll(filepath.Join(dir, "Default", "Cache"), 0o700)
	write(t, filepath.Join(dir, "Default", "Cookies"), "cookies")
	write(t, filepath.Join(dir, "Default", "Cache", "x"), "cache")
	os.MkdirAll(s.Dirs.Extension("ext-1"), 0o700)
	write(t, filepath.Join(s.Dirs.Extension("ext-1"), "manifest.json"), "{}")
	s.Store.CreateExtension(&store.Extension{ID: "ext-1", Name: "E", Version: "1"})
	os.MkdirAll(s.Dirs.Recipes(), 0o700)
	write(t, filepath.Join(s.Dirs.Recipes(), "r.md"), "recipe")
	return s, p.ID
}

func TestBackupToANewComputer(t *testing.T) {
	src, id := backupSource(t)
	folder, err := src.CreateFolder("Khách A", "blue")
	if err != nil {
		t.Fatal(err)
	}
	if err := src.SetProfileFolder(id, folder.ID); err != nil {
		t.Fatal(err)
	}
	tpl, err := src.CreateTemplate(store.Template{Name: "TikTok VN", Sites: []string{"tiktok"}, Languages: "vi-VN,vi", UILocale: "vi-VN", FolderID: folder.ID})
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "all.apbackup")
	res, err := src.ExportBackup(file, pw)
	if err != nil {
		t.Fatal(err)
	}
	if res.Profiles != 1 || res.Proxies != 1 || res.Accounts != 1 || res.Extensions != 1 || res.Bytes == 0 {
		t.Fatalf("export = %+v", res)
	}
	raw, _ := os.ReadFile(file)
	for _, s := range []string{"proxy-pw", "cookie-key", "site-pw", "cookies", "Kênh A"} {
		if bytes.Contains(raw, []byte(s)) {
			t.Fatalf("the backup shows %q in the clear", s)
		}
	}
	// Windows has no Unix modes: the file is the user's by its folder's ACL.
	if fi, _ := os.Stat(file); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode().Perm())
	}

	dst, sec := newService(t)
	ctx := context.Background()
	if _, err := dst.ImportBackup(ctx, file, "wrong password"); err == nil || !strings.Contains(err.Error(), "backup_wrong_password") {
		t.Fatalf("wrong password: %v", err)
	}
	got, err := dst.ImportBackup(ctx, file, pw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles != 1 || got.Proxies != 1 || got.Accounts != 1 || got.Extensions != 1 {
		t.Fatalf("import = %+v", got)
	}
	p, err := dst.Store.GetProfile(id)
	if err != nil || p.Name != "Kênh A" || p.ProxyID != "px1" || p.FolderID != folder.ID {
		t.Fatalf("profile = %+v, %v", p, err)
	}
	if f, err := dst.Store.GetFolder(folder.ID); err != nil || f.Name != folder.Name {
		t.Fatalf("folder=%+v err=%v", f, err)
	}
	if restored, err := dst.Store.GetTemplate(tpl.ID); err != nil || restored.FolderID != folder.ID || restored.Languages != tpl.Languages {
		t.Fatalf("template=%+v err=%v", restored, err)
	}
	if got := read(t, filepath.Join(dst.Dirs.Profile(id), "Default", "Cookies")); got != "cookies" {
		t.Fatalf("cookies file = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dst.Dirs.Profile(id), "Default", "Cache")); !os.IsNotExist(err) {
		t.Fatal("the cache came along")
	}
	for _, c := range []struct{ svc, acct, want string }{
		{secrets.ServiceProxy, "px1", "proxy-pw"}, {secrets.ServiceCookieKey, id, "cookie-key"},
		{secrets.ServiceAccountPassword, "acct1", "site-pw"}, {secrets.ServiceTOTP, "acct1", "totp"},
	} {
		if v, err := sec.Get(c.svc, c.acct); err != nil || string(v) != c.want {
			t.Fatalf("%s = %q, %v", c.svc, v, err)
		}
	}
	if got := read(t, filepath.Join(dst.Dirs.Extension("ext-1"), "manifest.json")); got != "{}" {
		t.Fatalf("extension = %q", got)
	}
	if got := read(t, filepath.Join(dst.Dirs.Recipes(), "r.md")); got != "recipe" {
		t.Fatalf("recipe = %q", got)
	}
	if left, _ := filepath.Glob(filepath.Join(dst.Dirs.Root, ".import-*")); len(left) != 0 {
		t.Fatalf("staging left behind: %v", left)
	}

	// Restoring again adds nothing and changes nothing.
	sec.Set(secrets.ServiceCookieKey, id, []byte("changed here"))
	again, err := dst.ImportBackup(ctx, file, pw)
	if err != nil || again.Profiles != 0 || again.ProfilesHere != 1 || again.ProxiesHere != 1 {
		t.Fatalf("again = %+v, %v", again, err)
	}
	if v, _ := sec.Get(secrets.ServiceCookieKey, id); string(v) != "changed here" {
		t.Fatalf("a key here was replaced: %q", v)
	}
}

func TestBackupRefuses(t *testing.T) {
	src, _ := backupSource(t)
	dir := t.TempDir()
	if _, err := src.ExportBackup(filepath.Join(dir, "a"), "short"); err == nil || !strings.Contains(err.Error(), "backup_password_short") {
		t.Fatalf("short password: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a")); !os.IsNotExist(err) {
		t.Fatal("a file was written")
	}
	other := filepath.Join(dir, "other")
	write(t, other, "not a backup at all, just some text")
	if _, err := src.ImportBackup(context.Background(), other, pw); err == nil || !strings.Contains(err.Error(), "backup_not_a_backup") {
		t.Fatalf("other file: %v", err)
	}
}

// A Keychain that refuses (the question cancelled) stops the backup before
// anything is written.
func TestBackupStopsWhenASecretCannotBeRead(t *testing.T) {
	src, _ := backupSource(t)
	src.Secrets = refusing{src.Secrets}
	file := filepath.Join(t.TempDir(), "all.apbackup")
	if _, err := src.ExportBackup(file, pw); err == nil {
		t.Fatal("want the Keychain's error")
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("a file was written")
	}
}

type refusing struct{ secrets.Store }

func (refusing) Get(string, string) ([]byte, error) {
	return nil, os.ErrPermission
}

func TestBackupRestoresProSettingsAndReencryptsCookies(t *testing.T) {
	src, sec := newService(t)
	src.Engines["anybrowser"] = fakeEngine2{}
	src.Preferred = "anybrowser"
	src.BrowserCookieKey = func() ([]byte, error) { return []byte("old-mac-safe-storage-pass"), nil }

	ctx := context.Background()
	if err := src.SetLanguage("vi"); err != nil {
		t.Fatal(err)
	}
	if err := src.SetMaxConcurrentLaunches(3); err != nil {
		t.Fatal(err)
	}
	_ = sec.Set(proVault, proAccount, []byte("APRO-11111-22222-33333-44444"))
	_ = sec.Set(proVault, proDeviceAcct, bytes.Repeat([]byte{9}, 32))
	_ = sec.Set(proVault, proCertAccount, []byte("cert-token"))

	p, err := src.CreateProfile(ctx, NewProfile{Name: "AnyBrowser Profile", Sites: []string{"other"}})
	if err != nil {
		t.Fatal(err)
	}
	jarDir := filepath.Join(src.Dirs.Profile(p.ID), "Default")
	if err := os.MkdirAll(jarDir, 0o700); err != nil {
		t.Fatal(err)
	}
	jarPath := filepath.Join(jarDir, "Cookies")
	srcBlock, err := aes.NewCipher(deriveChromiumMacKey([]byte("old-mac-safe-storage-pass")))
	if err != nil {
		t.Fatal(err)
	}
	wantPlain := []byte("0123456789abcdef0123456789abcdefSID=secret_session_value")
	encCookie := encryptChromiumV10(srcBlock, wantPlain)
	db, err := sql.Open("sqlite", "file:"+jarPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE cookies (name TEXT, encrypted_value BLOB); INSERT INTO cookies VALUES ('SID', ?)`, encCookie); err != nil {
		t.Fatal(err)
	}
	db.Close()

	file := filepath.Join(t.TempDir(), "move.apbackup")
	if _, err := src.ExportBackup(file, pw); err != nil {
		t.Fatal(err)
	}

	// Destination Mac has a different "Chromium Safe Storage" key in its Keychain.
	dst, dstSec := newService(t)
	dst.Engines["anybrowser"] = fakeEngine2{}
	dst.BrowserCookieKey = func() ([]byte, error) { return []byte("new-mac-safe-storage-pass"), nil }

	if _, err := dst.ImportBackup(ctx, file, pw); err != nil {
		t.Fatal(err)
	}
	if got := dst.Language(); got != "vi" {
		t.Fatalf("language = %q, want vi", got)
	}
	if got := dst.MaxConcurrentLaunches(); got != 3 {
		t.Fatalf("max concurrent launches = %d, want 3", got)
	}
	if k, _ := dstSec.Get(proVault, proAccount); string(k) != "APRO-11111-22222-33333-44444" {
		t.Fatalf("pro license = %q", k)
	}
	if c, _ := dstSec.Get(proVault, proCertAccount); string(c) != "cert-token" {
		t.Fatalf("pro cert = %q", c)
	}

	// Verify the cookie jar was re-encrypted to the destination Mac's key.
	dstJar := filepath.Join(dst.Dirs.Profile(p.ID), "Default", "Cookies")
	db2, err := sql.Open("sqlite", "file:"+dstJar)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	var gotEnc []byte
	if err := db2.QueryRow(`SELECT encrypted_value FROM cookies WHERE name = 'SID'`).Scan(&gotEnc); err != nil {
		t.Fatal(err)
	}
	dstBlock, err := aes.NewCipher(deriveChromiumMacKey([]byte("new-mac-safe-storage-pass")))
	if err != nil {
		t.Fatal(err)
	}
	plain, ok := decryptChromiumV10(dstBlock, gotEnc)
	if !ok || !bytes.Equal(plain, wantPlain) {
		t.Fatalf("decrypted cookie = %q (ok=%v), want %q", plain, ok, wantPlain)
	}
	if _, err := dstSec.Get(serviceChromiumSafeStorage, p.ID); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("temporary source browser key was not cleaned up: %v", err)
	}
}
