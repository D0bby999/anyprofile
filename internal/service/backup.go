// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/snapshot"
	"github.com/D0bby999/anyprofile/internal/store"
)

// A backup holds everything needed to carry AnyProfile to another computer:
// the database, every profile folder (without caches, as in a snapshot), the
// extensions and recipes, and the secrets (proxy passwords, cookie keys,
// account passwords and 2FA). The secrets matter most: the Keychain keeps
// them on this computer only, and a profile without its cookie key has lost
// its logins.
//
// Format: "APBACKUP1\n", a 16-byte salt, Argon2id time and memory (KiB) as
// uint32 big endian and threads as one byte, then a snapshot stream keyed
// with Argon2id(password, salt). Inside: manifest.json, anyprofile.db,
// secrets.json (never written to disk), profiles/<id>/, extensions/<id>/,
// recipes/.
const (
	backupMagic   = "APBACKUP1\n"
	backupTime    = 3
	backupMemory  = 64 << 10 // KiB
	backupThreads = 4
	minPassword   = 8
)

// BackupResult counts what a backup holds, or what a restore added.
type BackupResult struct {
	Profiles   int   `json:"profiles"`
	Proxies    int   `json:"proxies"`
	Accounts   int   `json:"accounts"`
	Extensions int   `json:"extensions"`
	Bytes      int64 `json:"bytes"` // the backup file
	// Restore only: already on this computer, left as they are.
	ProfilesHere int `json:"profilesHere"`
	ProxiesHere  int `json:"proxiesHere"`
}

type backupSecret struct {
	Service string `json:"service"`
	Account string `json:"account"`
	Value   []byte `json:"value"`
}

type backupManifest struct {
	Format  int    `json:"format"`
	Created string `json:"created"`
}

// safeID is what a profile or extension id may be before it names a folder.
var safeID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func backupKey(password string, salt []byte, t, m uint32, p uint8) []byte {
	return argon2.IDKey([]byte(password), salt, t, m, p, 32)
}

// ExportBackup writes a password-protected backup of everything to path.
// Open profiles must be closed first: a running browser rewrites its files.
func (s *Service) ExportBackup(path, password string) (BackupResult, error) {
	var res BackupResult
	if len([]rune(password)) < minPassword {
		return res, apperr.Newf("backup_password_short", "the password needs at least %d characters", minPassword)
	}
	if n := len(s.running()); n > 0 {
		return res, apperr.Newf("backup_profiles_open", "close the %d open profiles first", n)
	}
	var profiles []store.Profile
	for _, trashed := range []bool{false, true} {
		ps, err := s.Store.ListProfiles(store.ProfileFilter{Trashed: trashed})
		if err != nil {
			return res, err
		}
		profiles = append(profiles, ps...)
	}
	proxies, err := s.Store.ListProxies()
	if err != nil {
		return res, err
	}
	accounts, err := s.Store.AccountKeys()
	if err != nil {
		return res, err
	}
	exts, err := s.Store.ListExtensions()
	if err != nil {
		return res, err
	}

	// Secrets first: when the Keychain refuses, nothing is written.
	var sec []backupSecret
	defer func() {
		for _, x := range sec {
			secrets.Wipe(x.Value)
		}
	}()
	add := func(service, account string) error {
		v, err := s.Secrets.Get(service, account)
		if errors.Is(err, secrets.ErrNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", service, err)
		}
		sec = append(sec, backupSecret{service, account, v})
		return nil
	}
	for _, px := range proxies {
		if err := add(secrets.ServiceProxy, px.ID); err != nil {
			return res, err
		}
	}
	for _, p := range profiles {
		if err := add(secrets.ServiceCookieKey, p.ID); err != nil {
			return res, err
		}
	}
	for _, a := range accounts {
		if a.Password {
			if err := add(secrets.ServiceAccountPassword, a.SecretKey); err != nil {
				return res, err
			}
		}
		if a.TOTP {
			if err := add(secrets.ServiceTOTP, a.SecretKey); err != nil {
				return res, err
			}
		}
	}
	secJSON, err := json.Marshal(sec)
	if err != nil {
		return res, err
	}
	defer secrets.Wipe(secJSON)

	dbCopy := filepath.Join(s.Dirs.Root, ".backup-"+randomHex(4)+".db")
	if err := s.Store.CopyTo(dbCopy); err != nil {
		return res, err
	}
	db, err := os.ReadFile(dbCopy)
	os.Remove(dbCopy)
	if err != nil {
		return res, err
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return res, err
	}
	key := backupKey(password, salt, backupTime, backupMemory, backupThreads)
	defer secrets.Wipe(key)
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return res, err
	}
	n, err := s.writeBackup(f, salt, key, db, secJSON, profiles)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
		return res, err
	}
	res = BackupResult{Profiles: len(profiles), Proxies: len(proxies), Accounts: len(accounts), Extensions: len(exts), Bytes: n}
	s.Log.Info("backup written", "profiles", res.Profiles, "proxies", res.Proxies, "bytes", n)
	return res, nil
}

func (s *Service) writeBackup(w io.Writer, salt, key, db, secJSON []byte, profiles []store.Profile) (int64, error) {
	head := append([]byte(backupMagic), salt...)
	head = binary.BigEndian.AppendUint32(head, backupTime)
	head = binary.BigEndian.AppendUint32(head, backupMemory)
	head = append(head, backupThreads)
	if _, err := w.Write(head); err != nil {
		return 0, err
	}
	a, err := snapshot.NewArchive(w, key)
	if err != nil {
		return 0, err
	}
	man, _ := json.Marshal(backupManifest{Format: 1, Created: time.Now().UTC().Format(time.RFC3339)})
	for _, f := range []struct {
		name string
		data []byte
	}{{"manifest.json", man}, {"anyprofile.db", db}, {"secrets.json", secJSON}} {
		if err := a.AddFile(f.name, f.data); err != nil {
			return 0, err
		}
	}
	for _, p := range profiles {
		if !fileExists(s.Dirs.Profile(p.ID)) {
			continue // never opened
		}
		// The same lock as a snapshot: a launch waits until the copy is done.
		lock := s.lockFor(p.ID)
		if !lock.acquire(busyWait) {
			return 0, apperr.New("snapshot_busy", "a snapshot of this profile is in progress")
		}
		err := a.AddDir(s.Dirs.Profile(p.ID), "profiles/"+p.ID, skipInSnapshot)
		lock.release()
		if err != nil {
			return 0, err
		}
	}
	for _, d := range []struct{ dir, prefix string }{{s.Dirs.Extensions(), "extensions"}, {s.Dirs.Recipes(), "recipes"}} {
		if fileExists(d.dir) {
			if err := a.AddDir(d.dir, d.prefix, nil); err != nil {
				return 0, err
			}
		}
	}
	n, err := a.Close()
	return n + int64(len(head)), err
}

// ImportBackup adds what a backup holds and this computer does not have:
// profiles with their files, accounts and keys, proxies with their
// passwords, extensions, custom platforms and recipes. Nothing here is
// changed or removed, so restoring onto a computer in use is safe; a profile
// already here stays as it is. Settings stay this computer's.
func (s *Service) ImportBackup(ctx context.Context, path, password string) (BackupResult, error) {
	var res BackupResult
	f, err := os.Open(path)
	if err != nil {
		return res, err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	head := make([]byte, len(backupMagic)+16+9)
	if _, err := io.ReadFull(r, head); err != nil || string(head[:len(backupMagic)]) != backupMagic {
		return res, apperr.New("backup_not_a_backup", "this is not an AnyProfile backup")
	}
	salt := head[len(backupMagic) : len(backupMagic)+16]
	t := binary.BigEndian.Uint32(head[len(backupMagic)+16:])
	m := binary.BigEndian.Uint32(head[len(backupMagic)+20:])
	p := head[len(backupMagic)+24]
	if t == 0 || t > 10 || m < 8<<10 || m > 1<<20 || p == 0 || p > 16 {
		return res, apperr.New("backup_not_a_backup", "this is not an AnyProfile backup")
	}
	key := backupKey(password, salt, t, m, p)
	defer secrets.Wipe(key)

	if old, _ := filepath.Glob(filepath.Join(s.Dirs.Root, ".import-*")); len(old) > 0 {
		for _, o := range old {
			os.RemoveAll(o) // left by an import that was cut short
		}
	}
	staging := filepath.Join(s.Dirs.Root, ".import-"+randomHex(4))
	defer os.RemoveAll(staging)
	kept, err := snapshot.ReadKeeping(r, key, staging, func(n string) bool { return n == "secrets.json" })
	if errors.Is(err, snapshot.ErrCorrupt) {
		return res, apperr.New("backup_wrong_password", "wrong password, or the file is damaged")
	}
	if err != nil {
		return res, err
	}
	var sec []backupSecret
	defer func() {
		for _, x := range sec {
			secrets.Wipe(x.Value)
		}
		secrets.Wipe(kept["secrets.json"])
	}()
	if err := json.Unmarshal(kept["secrets.json"], &sec); err != nil {
		return res, apperr.New("backup_not_a_backup", "this is not an AnyProfile backup")
	}
	theirs := map[[2]string][]byte{}
	for _, x := range sec {
		theirs[[2]string{x.Service, x.Account}] = x.Value
	}

	// Bring the backup's database to this version's schema, then merge.
	dbPath := filepath.Join(staging, "anyprofile.db")
	bak, err := store.Open(dbPath, staging)
	if err != nil {
		return res, fmt.Errorf("read the backup's database: %w", err)
	}
	bak.Close()

	var moved [][2]string // from, to: put back if the merge fails
	var set [][2]string   // secrets added here
	undo := func() {
		for i := len(moved) - 1; i >= 0; i-- {
			os.Rename(moved[i][1], moved[i][0])
		}
		for _, k := range set {
			s.Secrets.Delete(k[0], k[1])
		}
	}
	setSecret := func(service, account string) error {
		v, ok := theirs[[2]string{service, account}]
		if !ok {
			return nil
		}
		if _, err := s.Secrets.Get(service, account); err == nil {
			return nil // never replace a secret this computer has
		} else if !errors.Is(err, secrets.ErrNotFound) {
			return err
		}
		if err := s.Secrets.Set(service, account, v); err != nil {
			return err
		}
		set = append(set, [2]string{service, account})
		return nil
	}
	move := func(from, to string) error {
		if !fileExists(from) {
			return nil
		}
		if fileExists(to) {
			return fmt.Errorf("%s is already there", to)
		}
		if err := os.MkdirAll(filepath.Dir(to), 0o700); err != nil {
			return err
		}
		if err := os.Rename(from, to); err != nil {
			return err
		}
		moved = append(moved, [2]string{from, to})
		return nil
	}
	merged, err := s.Store.Merge(ctx, dbPath, func(r store.MergeResult) error {
		for _, id := range append(append([]string(nil), r.Profiles...), r.Extensions...) {
			if !safeID.MatchString(id) {
				return fmt.Errorf("the backup holds a bad id %q", id)
			}
		}
		for _, id := range r.Profiles {
			if err := move(filepath.Join(staging, "profiles", id), s.Dirs.Profile(id)); err != nil {
				return err
			}
			if err := setSecret(secrets.ServiceCookieKey, id); err != nil {
				return err
			}
		}
		for _, id := range r.Extensions {
			if err := move(filepath.Join(staging, "extensions", id), s.Dirs.Extension(id)); err != nil {
				return err
			}
		}
		for _, id := range r.Proxies {
			if err := setSecret(secrets.ServiceProxy, id); err != nil {
				return err
			}
		}
		for _, a := range r.Accounts {
			if a.Password {
				if err := setSecret(secrets.ServiceAccountPassword, a.SecretKey); err != nil {
					return err
				}
			}
			if a.TOTP {
				if err := setSecret(secrets.ServiceTOTP, a.SecretKey); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if errors.Is(err, store.ErrNewerDatabase) {
		undo()
		return res, apperr.New("backup_newer", "the backup was made by a newer AnyProfile; update first")
	}
	if err != nil {
		undo()
		return res, err
	}
	s.importRecipes(filepath.Join(staging, "recipes"))
	res = BackupResult{Profiles: len(merged.Profiles), Proxies: len(merged.Proxies), Accounts: len(merged.Accounts),
		Extensions: len(merged.Extensions), ProfilesHere: merged.ProfilesHere, ProxiesHere: merged.ProxiesHere}
	s.Log.Info("backup restored", "profiles", res.Profiles, "proxies", res.Proxies, "profilesHere", res.ProfilesHere)
	for _, kind := range []string{"profiles", "proxies", "sites", "extensions"} {
		s.notify(kind)
	}
	return res, nil
}

// importRecipes adds the backup's recipes this computer does not have.
func (s *Service) importRecipes(from string) {
	entries, err := os.ReadDir(from)
	if err != nil {
		return
	}
	if err := os.MkdirAll(s.Dirs.Recipes(), 0o700); err != nil {
		return
	}
	for _, e := range entries {
		to := filepath.Join(s.Dirs.Recipes(), e.Name())
		if !fileExists(to) {
			os.Rename(filepath.Join(from, e.Name()), to)
		}
	}
}
