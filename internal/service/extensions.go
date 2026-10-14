// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"errors"
	"os"
	"slices"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/bookmarks"
	"github.com/D0bby999/anyprofile/internal/extension"
	"github.com/D0bby999/anyprofile/internal/store"
)

// AddExtension unpacks a folder, .zip or .crx into the library.
func (s *Service) AddExtension(path string) (store.Extension, error) {
	id := "ext-" + randomHex(6)
	m, err := extension.Unpack(path, s.Dirs.Extension(id))
	if errors.Is(err, extension.ErrNoManifest) {
		return store.Extension{}, apperr.New("extension_no_manifest", "the folder or file has no valid manifest.json")
	}
	if err != nil {
		return store.Extension{}, apperr.New("extension_unreadable", err.Error())
	}
	e := store.Extension{ID: id, Name: m.Name, Version: m.Version}
	if err := s.Store.CreateExtension(&e); err != nil {
		os.RemoveAll(s.Dirs.Extension(id))
		return store.Extension{}, err
	}
	s.Log.Info("extension added", "id", id, "name", m.Name, "version", m.Version)
	s.notify("extensions")
	return e, nil
}

func (s *Service) ListExtensions() ([]store.Extension, error) { return s.Store.ListExtensions() }

// DeleteExtension removes it from the library and from every profile. A
// profile running with it keeps it until it closes, so that is refused.
func (s *Service) DeleteExtension(id string) error {
	rows, err := s.Store.ListProfiles(store.ProfileFilter{})
	if err != nil {
		return err
	}
	for _, p := range rows {
		if _, running := s.Launcher.Info(p.ID); running && slices.Contains(p.Extensions, id) {
			return apperr.New("extension_in_use", "close the profiles that use this extension first")
		}
	}
	if err := s.Store.DeleteExtension(id); err != nil {
		return err
	}
	if err := os.RemoveAll(s.Dirs.Extension(id)); err != nil {
		return err
	}
	s.notify("extensions")
	s.notify("profiles")
	return nil
}

// SetExtensionMany turns one extension on or off for several profiles. It
// applies from each profile's next launch.
func (s *Service) SetExtensionMany(ids []string, extID string, on bool) error {
	exts, err := s.Store.ListExtensions()
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(exts, func(e store.Extension) bool { return e.ID == extID }) {
		return apperr.New("extension_unknown", "that extension is not in the library")
	}
	defer s.notify("profiles") // also after a partial failure: earlier rows did change
	defer s.notify("extensions")
	for _, id := range ids {
		if _, err := s.Store.GetProfile(id); err != nil {
			return err
		}
		if err := s.Store.SetProfileExtension(id, extID, on); err != nil {
			return err
		}
	}
	return nil
}

// ImportBookmarks adds a bookmark HTML file to a closed profile, in a folder
// on its bookmarks bar. It returns how many links were added.
func (s *Service) ImportBookmarks(profileID, path, folder string) (int, error) {
	if _, err := s.Store.GetProfile(profileID); err != nil {
		return 0, err
	}
	if _, running := s.Launcher.Info(profileID); running {
		return 0, apperr.New("bookmarks_need_closed", "close the profile first; the browser rewrites its bookmarks when it quits")
	}
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	if st.Size() > bookmarks.MaxFileBytes {
		return 0, apperr.New("bookmarks_too_big", "the file is larger than 5 MB")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	root, n, err := bookmarks.Parse(b)
	if err != nil {
		return 0, apperr.New("bookmarks_unreadable", err.Error())
	}
	if err := bookmarks.AddToProfile(s.Dirs.Profile(profileID), folder, root); err != nil {
		return 0, err
	}
	s.Log.Info("bookmarks imported", "profile", profileID, "count", n)
	return n, nil
}
