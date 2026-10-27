// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/store"
)

// FolderColors is the fixed palette a folder color comes from; "" is no
// color. The UI renders these names, the backend only validates them.
var FolderColors = map[string]bool{
	"": true, "gray": true, "red": true, "orange": true, "yellow": true,
	"green": true, "teal": true, "blue": true, "purple": true, "pink": true,
}

// FolderRow is a folder with its live profile count, for the sidebar.
type FolderRow struct {
	store.Folder
	Count int `json:"count"`
}

// ListFolders returns every folder with its profile count, plus the
// number of unfiled profiles.
func (s *Service) ListFolders() ([]FolderRow, int, error) {
	fs, err := s.Store.ListFolders()
	if err != nil {
		return nil, 0, err
	}
	counts, err := s.Store.FolderCounts()
	if err != nil {
		return nil, 0, err
	}
	out := make([]FolderRow, len(fs))
	for i, f := range fs {
		out[i] = FolderRow{Folder: f, Count: counts[f.ID]}
	}
	return out, counts[""], nil
}

// CreateFolder makes a folder; the name must be unique (trimmed,
// case-insensitive) and the color from the fixed palette.
func (s *Service) CreateFolder(name, color string) (store.Folder, error) {
	if !FolderColors[color] {
		return store.Folder{}, apperr.Newf("bad_folder_color", "unknown folder color %q", color)
	}
	f := &store.Folder{ID: randomHex(8), Name: name, Color: color}
	if err := s.Store.CreateFolder(f); err != nil {
		if err == store.ErrFolderNameTaken {
			return store.Folder{}, apperr.Newf("folder_name_taken", "a folder named %q already exists", name)
		}
		return store.Folder{}, err
	}
	s.notify("profiles")
	return *f, nil
}

// RenameFolder renames a folder under the same uniqueness rule.
func (s *Service) RenameFolder(id, name string) error {
	if err := s.Store.RenameFolder(id, name); err != nil {
		if err == store.ErrFolderNameTaken {
			return apperr.Newf("folder_name_taken", "a folder named %q already exists", name)
		}
		return err
	}
	s.notify("profiles")
	return nil
}

// SetFolderColor recolors a folder; the color must be in the palette.
func (s *Service) SetFolderColor(id, color string) error {
	if !FolderColors[color] {
		return apperr.Newf("bad_folder_color", "unknown folder color %q", color)
	}
	if err := s.Store.SetFolderColor(id, color); err != nil {
		return err
	}
	s.notify("profiles")
	return nil
}

// ReorderFolders persists the sidebar order of folder ids.
func (s *Service) ReorderFolders(ids []string) error {
	if err := s.Store.ReorderFolders(ids); err != nil {
		return err
	}
	s.notify("profiles")
	return nil
}

// DeleteFolder removes a folder after moving its profiles to moveTo
// ("" unfiles them). Profiles are never deleted with a folder.
func (s *Service) DeleteFolder(id, moveTo string) error {
	if err := s.Store.DeleteFolder(id, moveTo); err != nil {
		return err
	}
	s.notify("profiles")
	return nil
}

// SetProfileFolder files one profile, or unfiles it when folderID is "".
// A non-empty folder must exist.
func (s *Service) SetProfileFolder(profileID, folderID string) error {
	if folderID != "" {
		if _, err := s.Store.GetFolder(folderID); err != nil {
			return err
		}
	}
	if err := s.Store.SetProfileFolder(profileID, folderID); err != nil {
		return err
	}
	s.notify("profiles")
	return nil
}

// MoveProfilesToFolder files many profiles at once (the bulk action);
// it reports how many were moved.
func (s *Service) MoveProfilesToFolder(ids []string, folderID string) (int, error) {
	if folderID != "" {
		if _, err := s.Store.GetFolder(folderID); err != nil {
			return 0, err
		}
	}
	moved := 0
	for _, id := range ids {
		if err := s.Store.SetProfileFolder(id, folderID); err != nil {
			return moved, err
		}
		moved++
	}
	if moved > 0 {
		s.notify("profiles")
	}
	return moved, nil
}
