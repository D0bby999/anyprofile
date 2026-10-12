// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"encoding/json"
	"fmt"
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

// DeleteCustomSite removes the site and moves its profiles to "other".
func (s *Store) DeleteCustomSite(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE profiles SET site = 'other', updated_at = ? WHERE site = ?`, now(), id); err != nil {
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
