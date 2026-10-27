// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

// Template is a reusable set of profile-creation defaults for one
// platform. ProxyNote is a recommendation shown to the user ("residential
// VN, sticky"), never a proxy: templates hold no secrets, no concrete
// proxies and no cookies, by construction — there is no field for them.
type Template struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Site      string   `json:"site"`
	Sites     []string `json:"sites"`
	ProxyNote string   `json:"proxyNote"`
	Languages string   `json:"languages"`
	UILocale  string   `json:"uiLocale"`
	Engine    string   `json:"engine"`
	StartURL  string   `json:"startUrl"`
	Notes     string   `json:"notes"`
	Tags      []string `json:"tags"`
	FolderID  string   `json:"folderId"`
	// Builtin marks the code-defined presets; they are not stored and
	// cannot be deleted, only copied into a user template.
	Builtin   bool   `json:"builtin"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

const templateCols = `id, name, site, sites, proxy_note, languages, ui_locale,
	engine, start_url, notes, tags, folder_id, created_at, updated_at`

func scanTemplate(row interface{ Scan(...any) error }) (Template, error) {
	var t Template
	var sites, tags string
	err := row.Scan(&t.ID, &t.Name, &t.Site, &sites, &t.ProxyNote, &t.Languages,
		&t.UILocale, &t.Engine, &t.StartURL, &t.Notes, &tags, &t.FolderID,
		&t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return t, err
	}
	_ = json.Unmarshal([]byte(sites), &t.Sites)
	_ = json.Unmarshal([]byte(tags), &t.Tags)
	if t.Sites == nil {
		t.Sites = []string{}
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	return t, nil
}

// ListTemplates returns the user's templates in name order.
func (s *Store) ListTemplates() ([]Template, error) {
	rows, err := s.db.Query(`SELECT ` + templateCols + ` FROM templates ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Template{}
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetTemplate returns one user template, or ErrNotFound.
func (s *Store) GetTemplate(id string) (Template, error) {
	t, err := scanTemplate(s.db.QueryRow(`SELECT `+templateCols+` FROM templates WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

// CreateTemplate stores a new user template.
func (s *Store) CreateTemplate(t *Template) error {
	t.Name = strings.TrimSpace(t.Name)
	if t.ID == "" || t.Name == "" {
		return errors.New("template needs id and name")
	}
	t.CreatedAt, t.UpdatedAt = now(), now()
	_, err := s.db.Exec(`INSERT INTO templates (`+templateCols+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.Name, t.Site, tagsJSON(t.Sites), t.ProxyNote, t.Languages, t.UILocale,
		t.Engine, t.StartURL, t.Notes, tagsJSON(t.Tags), t.FolderID, t.CreatedAt, t.UpdatedAt)
	return err
}

// UpdateTemplate replaces a user template's fields.
func (s *Store) UpdateTemplate(t *Template) error {
	t.Name = strings.TrimSpace(t.Name)
	if t.Name == "" {
		return errors.New("template name must not be empty")
	}
	t.UpdatedAt = now()
	res, err := s.db.Exec(`UPDATE templates SET name=?, site=?, sites=?, proxy_note=?, languages=?,
		ui_locale=?, engine=?, start_url=?, notes=?, tags=?, folder_id=?, updated_at=? WHERE id=?`,
		t.Name, t.Site, tagsJSON(t.Sites), t.ProxyNote, t.Languages, t.UILocale,
		t.Engine, t.StartURL, t.Notes, tagsJSON(t.Tags), t.FolderID, t.UpdatedAt, t.ID)
	return affected(res, err)
}

// DeleteTemplate removes a user template.
func (s *Store) DeleteTemplate(id string) error {
	res, err := s.db.Exec(`DELETE FROM templates WHERE id = ?`, id)
	return affected(res, err)
}
