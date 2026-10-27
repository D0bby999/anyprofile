// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import "github.com/D0bby999/anyprofile/internal/store"

// ProfilePage is one page of the profile list plus the total the filter
// matches, so the UI can page without holding the whole list.
type ProfilePage struct {
	Rows  []ProfileRow `json:"rows"`
	Total int          `json:"total"`
}

// ListProfilesPage returns at most limit rows starting at offset, with
// the full count of matching profiles. A limit of 0 returns everything,
// exactly like ListProfiles. Each row still carries its accounts and
// extensions, but the store now loads those only for the page's rows.
func (s *Service) ListProfilesPage(f store.ProfileFilter, limit, offset int) (ProfilePage, error) {
	total, err := s.Store.CountProfiles(f)
	if err != nil {
		return ProfilePage{}, err
	}
	f.Limit, f.Offset = limit, offset
	rows, err := s.ListProfiles(f)
	if err != nil {
		return ProfilePage{}, err
	}
	return ProfilePage{Rows: rows, Total: total}, nil
}
