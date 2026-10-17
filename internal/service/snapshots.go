// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/snapshot"
	"github.com/D0bby999/anyprofile/internal/store"
)

const (
	// autoSnapshotAfter is the shortest session that gets a snapshot on close.
	autoSnapshotAfter = 2 * time.Minute
	defaultKeep       = 5
	// keepOther is how many manual, before-restore and before-import snapshots stay.
	keepOther = 3
	// busyWait is how long a launch waits for a snapshot of its profile.
	busyWait = 60 * time.Second
)

// snapshotSkip are folders and files a snapshot leaves out: Chromium caches
// (rebuilt on demand, the sites' Cache API included), lock files of a running
// browser, crash reports, and components Chromium downloads again by itself.
// What stays (cookies, storage, history, settings) is a few MB; with the rest
// a close snapshot was ~30 MB.
var snapshotSkip = append(append(append(append([]string(nil), cacheDirs...), componentDirs...), downloadCaches...),
	"SingletonLock", "SingletonCookie", "SingletonSocket", "lockfile", "DevToolsActivePort",
	"Crashpad", "Default/Service Worker/CacheStorage")

func skipInSnapshot(rel string, _ fs.DirEntry) bool {
	for _, s := range snapshotSkip {
		if rel == s {
			return true
		}
	}
	return false
}

// SnapshotSettings are the automatic snapshot preferences.
type SnapshotSettings struct {
	Auto bool `json:"auto"`
	Keep int  `json:"keep"` // 1..10 snapshots taken on close
}

// profileLock is a one-slot semaphore: unlike a mutex, waiting on it can time out.
type profileLock chan struct{}

func (s *Service) lockFor(id string) profileLock {
	s.snapMu.Lock()
	defer s.snapMu.Unlock()
	if s.snapLocks == nil {
		s.snapLocks = map[string]profileLock{}
	}
	l, ok := s.snapLocks[id]
	if !ok {
		l = make(profileLock, 1)
		s.snapLocks[id] = l
	}
	return l
}

func (l profileLock) acquire(wait time.Duration) bool {
	select {
	case l <- struct{}{}:
		return true
	case <-time.After(wait):
		return false
	}
}

func (l profileLock) release() { <-l }

func (s *Service) SnapshotSettings() SnapshotSettings {
	st := SnapshotSettings{Auto: true, Keep: defaultKeep}
	if v, err := s.Store.GetSetting("snapshots_auto"); err == nil && v == "off" {
		st.Auto = false
	}
	if v, err := s.Store.GetSetting("snapshots_keep"); err == nil {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 10 {
			st.Keep = n
		}
	}
	return st
}

func (s *Service) SetSnapshotSettings(st SnapshotSettings) error {
	if st.Keep < 1 || st.Keep > 10 {
		return apperr.New("bad_keep", "keep between 1 and 10 snapshots")
	}
	auto := "on"
	if !st.Auto {
		auto = "off"
	}
	if err := s.Store.SetSetting("snapshots_auto", auto); err != nil {
		return err
	}
	if err := s.Store.SetSetting("snapshots_keep", strconv.Itoa(st.Keep)); err != nil {
		return err
	}
	s.notify("settings")
	return nil
}

// snapshotKey reads the snapshot key, creating it on first use.
func (s *Service) snapshotKey() ([]byte, error) {
	k, err := s.Secrets.Get(secrets.ServiceSnapshotKey, "app")
	if err == nil && len(k) == 32 {
		return k, nil
	}
	if err != nil && !errors.Is(err, secrets.ErrNotFound) {
		return nil, err
	}
	k = make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		return nil, err
	}
	return k, s.Secrets.Set(secrets.ServiceSnapshotKey, "app", k)
}

// TakeSnapshot saves an encrypted copy of a closed profile.
func (s *Service) TakeSnapshot(id, reason string) (store.Snapshot, error) {
	if s.running()[id] {
		return store.Snapshot{}, apperr.New("profile_running", "close the profile first")
	}
	lock := s.lockFor(id)
	if !lock.acquire(busyWait) {
		return store.Snapshot{}, apperr.New("snapshot_busy", "a snapshot of this profile is in progress")
	}
	defer lock.release()
	return s.takeLocked(id, reason)
}

func (s *Service) takeLocked(id, reason string) (store.Snapshot, error) {
	if _, err := os.Stat(s.Dirs.Profile(id)); err != nil {
		return store.Snapshot{}, apperr.New("nothing_to_snapshot", "the profile has never been opened")
	}
	key, err := s.snapshotKey()
	if err != nil {
		return store.Snapshot{}, err
	}
	defer secrets.Wipe(key)
	dir := s.Dirs.ProfileSnapshots(id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return store.Snapshot{}, err
	}
	name := time.Now().UTC().Format("20060102-150405") + "-" + reason + ".apsnap"
	for i := 2; fileExists(filepath.Join(dir, name)); i++ {
		name = fmt.Sprintf("%s-%s-%d.apsnap", time.Now().UTC().Format("20060102-150405"), reason, i)
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return store.Snapshot{}, err
	}
	n, err := snapshot.Write(tmp, s.Dirs.Profile(id), key, skipInSnapshot)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), filepath.Join(dir, name))
	}
	if err != nil {
		os.Remove(tmp.Name())
		return store.Snapshot{}, err
	}
	snap := store.Snapshot{ProfileID: id, Reason: reason, Bytes: n, File: name}
	if err := s.Store.AddSnapshot(&snap); err != nil {
		os.Remove(filepath.Join(dir, name))
		return store.Snapshot{}, err
	}
	keep := keepOther
	if reason == "close" {
		keep = s.SnapshotSettings().Keep
	}
	if old, err := s.Store.SnapshotsToPrune(id, reason, keep); err == nil {
		for _, o := range old {
			s.removeSnapshot(o)
		}
	}
	s.Log.Info("snapshot saved", "profile", id, "reason", reason, "bytes", n)
	s.notify("snapshots")
	return snap, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func (s *Service) removeSnapshot(x store.Snapshot) {
	os.Remove(filepath.Join(s.Dirs.ProfileSnapshots(x.ProfileID), x.File))
	if err := s.Store.DeleteSnapshot(x.ID); err != nil {
		s.Log.Warn("delete snapshot", "id", x.ID, "err", err)
	}
}

// RestoreSnapshot replaces a closed profile's folder with a snapshot. The
// current folder is saved first; its snapshot id is returned for undo.
func (s *Service) RestoreSnapshot(snapshotID int64) (int64, error) {
	snap, err := s.Store.GetSnapshot(snapshotID)
	if err != nil {
		return 0, err
	}
	id := snap.ProfileID
	if s.running()[id] {
		return 0, apperr.New("profile_running", "close the profile first")
	}
	lock := s.lockFor(id)
	if !lock.acquire(busyWait) {
		return 0, apperr.New("snapshot_busy", "a snapshot of this profile is in progress")
	}
	defer lock.release()

	var before int64
	if fileExists(s.Dirs.Profile(id)) {
		b, err := s.takeLocked(id, "before-restore")
		if err != nil {
			return 0, err
		}
		before = b.ID
	}
	key, err := s.snapshotKey()
	if err != nil {
		return before, err
	}
	defer secrets.Wipe(key)
	staging := filepath.Join(s.Dirs.Profiles(), ".restore-"+id)
	old := filepath.Join(s.Dirs.Profiles(), ".old-"+id)
	os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return before, err
	}
	f, err := os.Open(filepath.Join(s.Dirs.ProfileSnapshots(id), snap.File))
	if err != nil {
		os.RemoveAll(staging)
		return before, err
	}
	err = snapshot.Read(f, key, staging)
	f.Close()
	if err != nil {
		os.RemoveAll(staging)
		return before, apperr.Newf("snapshot_damaged", "%v", err)
	}
	cur := s.Dirs.Profile(id)
	hadCur := fileExists(cur)
	if hadCur {
		// Components are not in the snapshot: keep the ones on disk rather
		// than have the browser download them again.
		for _, c := range componentDirs {
			if !fileExists(filepath.Join(staging, c)) {
				os.Rename(filepath.Join(cur, c), filepath.Join(staging, c))
			}
		}
		os.RemoveAll(old)
		if err := os.Rename(cur, old); err != nil {
			os.RemoveAll(staging)
			return before, err
		}
	}
	if err := os.Rename(staging, cur); err != nil {
		if hadCur {
			os.Rename(old, cur) // put the profile back as it was
		}
		os.RemoveAll(staging)
		return before, err
	}
	os.RemoveAll(old)
	s.Log.Info("snapshot restored", "profile", id, "snapshot", snapshotID)
	s.notify("snapshots")
	s.notify("profiles")
	return before, nil
}

func (s *Service) Snapshots(profileID string) ([]store.Snapshot, error) {
	return s.Store.Snapshots(profileID)
}

func (s *Service) DeleteSnapshot(id int64) error {
	x, err := s.Store.GetSnapshot(id)
	if err != nil {
		return err
	}
	s.removeSnapshot(x)
	s.notify("snapshots")
	return nil
}

func (s *Service) DeleteAllSnapshots() error {
	all, err := s.Store.AllSnapshots()
	if err != nil {
		return err
	}
	for _, x := range all {
		s.removeSnapshot(x)
	}
	s.notify("snapshots")
	return nil
}

// BeforeLaunch waits for a snapshot of the profile to finish (Launcher.BeforeStart).
func (s *Service) BeforeLaunch(profileID string) error {
	lock := s.lockFor(profileID)
	if !lock.acquire(busyWait) {
		return apperr.New("snapshot_busy", "a snapshot of this profile is in progress")
	}
	lock.release()
	return nil
}

// AfterExit drops the profile's download caches and snapshots a profile that
// ran long enough (Launcher.AfterExit). The lock is taken before returning,
// so a launch right after waits for it.
func (s *Service) AfterExit(profileID string, ranFor time.Duration) {
	lock := s.lockFor(profileID)
	if !lock.acquire(busyWait) {
		return
	}
	go func() {
		defer lock.release()
		for _, c := range downloadCaches {
			os.RemoveAll(filepath.Join(s.Dirs.Profile(profileID), c))
		}
		if ranFor < autoSnapshotAfter || !s.SnapshotSettings().Auto {
			return
		}
		if _, err := s.takeLocked(profileID, "close"); err != nil {
			s.Log.Warn("snapshot on close", "profile", profileID, "err", err)
		}
	}()
}
