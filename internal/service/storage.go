// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

// Orphan is a profile folder with no profile row: left by a crash or a reset database.
type Orphan struct {
	ID    string `json:"id"`
	Bytes int64  `json:"bytes"`
}

type StorageInfo struct {
	ProfilesBytes  int64    `json:"profilesBytes"`
	SnapshotsBytes int64    `json:"snapshotsBytes"`
	Orphans        []Orphan `json:"orphans"`
}

// cacheDirs are Chromium caches, safe to delete while the browser is closed.
var cacheDirs = []string{"Default/Cache", "Default/Code Cache", "Default/GPUCache",
	"GrShaderCache", "ShaderCache", "GraphiteDawnCache"}

// dirSize sums the size of the regular files under path; errors are ignored
// (a vanished file mid-walk is simply not counted).
func dirSize(path string) int64 {
	var total int64
	filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if info, err := d.Info(); err == nil && d.Type().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// knownIDs is every profile row, live and trashed.
func (s *Service) knownIDs() (map[string]bool, error) {
	ids := map[string]bool{}
	for _, f := range []store.ProfileFilter{{}, {Trashed: true}} {
		ps, err := s.Store.ListProfiles(f)
		if err != nil {
			return nil, err
		}
		for _, p := range ps {
			ids[p.ID] = true
		}
	}
	return ids, nil
}

// orphanIDs lists the folders in the profiles directory that no profile row owns.
func (s *Service) orphanIDs() ([]string, error) {
	known, err := s.knownIDs()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.Dirs.Profiles())
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		// ".restore-x" and ".old-x" are a restore in progress, not orphans.
		if e.IsDir() && !known[e.Name()] && !strings.HasPrefix(e.Name(), ".") {
			ids = append(ids, e.Name())
		}
	}
	return ids, nil
}

func (s *Service) running() map[string]bool {
	m := map[string]bool{}
	for _, in := range s.Launcher.List() {
		m[in.ProfileID] = true
	}
	return m
}

// Storage measures the profiles directory and finds orphan folders.
func (s *Service) Storage() (StorageInfo, error) {
	ids, err := s.orphanIDs()
	if err != nil {
		return StorageInfo{}, err
	}
	info := StorageInfo{Orphans: []Orphan{}, ProfilesBytes: dirSize(s.Dirs.Profiles())}
	info.SnapshotsBytes, _ = s.Store.SnapshotBytes()
	for _, id := range ids {
		info.Orphans = append(info.Orphans, Orphan{ID: id, Bytes: dirSize(s.Dirs.Profile(id))})
	}
	s.Log.Info("storage measured", "profilesBytes", info.ProfilesBytes, "orphans", len(info.Orphans))
	return info, nil
}

// DeleteOrphans removes the listed folders and their Keychain items. Only ids
// a fresh scan reports as orphans are touched, so no id can reach the
// profiles root, a live profile or anything outside it.
func (s *Service) DeleteOrphans(ids []string) error {
	scanned, err := s.orphanIDs()
	if err != nil {
		return err
	}
	orphan := map[string]bool{}
	for _, id := range scanned {
		orphan[id] = true
	}
	run := s.running()
	var siteIDs []string
	if all, err := s.Sites(); err == nil {
		for _, x := range all {
			siteIDs = append(siteIDs, x.ID)
		}
	}
	deleted := 0
	var bytes int64
	for _, id := range ids {
		if !orphan[id] || run[id] {
			continue
		}
		p := s.Dirs.Profile(id)
		bytes += dirSize(p)
		if err := os.RemoveAll(p); err != nil {
			return err
		}
		// The rows are gone, so every platform's key is tried; deleting a
		// missing item is not an error.
		s.Secrets.Delete(secrets.ServiceCookieKey, id)
		s.deleteAccountSecrets(id)
		for _, site := range siteIDs {
			s.deleteAccountSecrets(id + "/" + site)
		}
		deleted++
	}
	s.Log.Info("orphans deleted", "count", deleted, "bytes", bytes)
	return nil
}

// ClearCache frees the Chromium caches of every profile that is not running.
// Cookies and logins are kept. It returns the number of bytes removed.
func (s *Service) ClearCache() (int64, error) {
	known, err := s.knownIDs()
	if err != nil {
		return 0, err
	}
	run := s.running()
	var total int64
	cleared := 0
	for id := range known {
		if run[id] {
			continue
		}
		for _, d := range cacheDirs {
			p := filepath.Join(s.Dirs.Profile(id), d)
			total += dirSize(p)
			if err := os.RemoveAll(p); err != nil {
				return total, err
			}
		}
		cleared++
	}
	s.Log.Info("cache cleared", "profiles", cleared, "bytes", total)
	return total, nil
}
