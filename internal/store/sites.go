// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
)

// CustomSite is a platform the user added. ID is "custom-<hex>".
type CustomSite struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Domains   []string `json:"domains"`
	StartURL  string   `json:"startUrl"`
	Color     string   `json:"color"`
	CreatedAt string   `json:"createdAt"`
}

func (s *Store) ListCustomSites() ([]CustomSite, error) {
	rows, err := s.db.Query(`SELECT id, name, domains, start_url, color, created_at
		FROM custom_sites ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CustomSite{}
	for rows.Next() {
		var c CustomSite
		var domains string
		if err := rows.Scan(&c.ID, &c.Name, &domains, &c.StartURL, &c.Color, &c.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(domains), &c.Domains); err != nil {
			return nil, fmt.Errorf("custom site %s domains: %w", c.ID, err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CreateCustomSite(c *CustomSite) error {
	c.CreatedAt = now()
	_, err := s.db.Exec(`INSERT INTO custom_sites (id, name, domains, start_url, color, created_at)
		VALUES (?,?,?,?,?,?)`, c.ID, c.Name, tagsJSON(c.Domains), c.StartURL, c.Color, c.CreatedAt)
	return err
}

func (s *Store) UpdateCustomSite(c CustomSite) error {
	res, err := s.db.Exec(`UPDATE custom_sites SET name=?, domains=?, start_url=?, color=? WHERE id=?`,
		c.Name, tagsJSON(c.Domains), c.StartURL, c.Color, c.ID)
	return affected(res, err)
}

// DeleteCustomSite removes the site from every profile (one left with none
// becomes "other"), then the site itself.
func (s *Store) DeleteCustomSite(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := dropSiteFromProfiles(tx, id); err != nil {
		tx.Rollback()
		return err
	}
	res, err := tx.Exec(`DELETE FROM custom_sites WHERE id = ?`, id)
	if err != nil {
		tx.Rollback()
		return err
	}
	if err := affected(res, nil); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func dropSiteFromProfiles(tx *sql.Tx, id string) error {
	rows, err := tx.Query(`SELECT id, sites FROM profiles WHERE EXISTS (SELECT 1 FROM json_each(sites) WHERE value = ?)`, id)
	if err != nil {
		return err
	}
	type change struct{ id, sites, first string }
	var changes []change
	for rows.Next() {
		var pid, raw string
		var ids []string
		if err := rows.Scan(&pid, &raw); err != nil {
			rows.Close()
			return err
		}
		if err := json.Unmarshal([]byte(raw), &ids); err != nil {
			rows.Close()
			return fmt.Errorf("profile %s sites: %w", pid, err)
		}
		left := normalSites(slices.DeleteFunc(ids, func(x string) bool { return x == id }))
		changes = append(changes, change{pid, tagsJSON(left), left[0]})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, c := range changes {
		if _, err := tx.Exec(`UPDATE profiles SET site = ?, sites = ?, updated_at = ? WHERE id = ?`, c.first, c.sites, now(), c.id); err != nil {
			return err
		}
	}
	return nil
}
