// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Profile is one browser identity. Engine, PersonaID and SeedHex are fixed at
// creation: changing them would change the "machine" a logged-in account sees.
type Profile struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Engine        string     `json:"engine"`
	EngineVersion string     `json:"engineVersion"`
	EngineMeta    EngineMeta `json:"engineMeta"`
	PersonaID     string     `json:"personaId"`
	SeedHex       string     `json:"seedHex"`
	Platform      string     `json:"platform"`
	ProxyID       string     `json:"proxyId"`
	Timezone      string     `json:"timezone"`
	Languages     string     `json:"languages"`
	UILocale      string     `json:"uiLocale"`
	Lat           *float64   `json:"lat"`
	Lon           *float64   `json:"lon"`
	Site          string     `json:"site"`
	Serial        int        `json:"serial"`
	Login         string     `json:"login"`
	HasPassword   bool       `json:"hasPassword"`
	HasTOTP       bool       `json:"hasTotp"`
	FPVerdict     string     `json:"fpVerdict"`
	FPDiff        []string   `json:"fpDiff"`
	FPCheckedAt   string     `json:"fpCheckedAt"`
	// Machine is the emulated machine from the fingerprint baseline, for
	// example "Apple M4 Pro · 1512×982"; empty before the first measurement.
	Machine       string   `json:"machine"`
	StartURL      string   `json:"startUrl"`
	Notes         string   `json:"notes"`
	Tags          []string `json:"tags"`
	CDPNextLaunch bool     `json:"cdpNextLaunch"`
	Pinned        bool     `json:"pinned"`
	CreatedAt     string   `json:"createdAt"`
	UpdatedAt     string   `json:"updatedAt"`
	LastOpenAt    string   `json:"lastOpenAt"`
	DeletedAt     string   `json:"deletedAt"`
}

// EngineMeta holds what an engine pins at creation. Zero values mean "not pinned".
type EngineMeta struct {
	HWC    int `json:"hwc,omitempty"`    // navigator.hardwareConcurrency
	MemGiB int `json:"memGiB,omitempty"` // installed memory in GiB
}

// ProfileFilter narrows ListProfiles.
type ProfileFilter struct {
	Query   string `json:"query"` // substring of the name, case-insensitive
	Tag     string `json:"tag"`
	Trashed bool   `json:"trashed"` // list the trash instead of live profiles
}

const profileCols = `id, name, engine, engine_version, persona_id, seed_hex, platform,
	COALESCE(proxy_id, ''), timezone, languages, ui_locale, lat, lon, site, start_url,
	notes, tags, cdp_next_launch, pinned, created_at, updated_at, COALESCE(last_open_at, ''),
	COALESCE(deleted_at, ''), serial, login, has_password, has_totp,
	fp_verdict, fp_diff, fp_checked_at, engine_meta, fp_baseline`

func scanProfile(row interface{ Scan(...any) error }) (Profile, error) {
	var p Profile
	var tags string
	var fpDiff, meta, baseline string
	err := row.Scan(&p.ID, &p.Name, &p.Engine, &p.EngineVersion, &p.PersonaID, &p.SeedHex,
		&p.Platform, &p.ProxyID, &p.Timezone, &p.Languages, &p.UILocale, &p.Lat, &p.Lon,
		&p.Site, &p.StartURL, &p.Notes, &tags, &p.CDPNextLaunch, &p.Pinned,
		&p.CreatedAt, &p.UpdatedAt, &p.LastOpenAt, &p.DeletedAt, &p.Serial, &p.Login,
		&p.HasPassword, &p.HasTOTP, &p.FPVerdict, &fpDiff, &p.FPCheckedAt, &meta, &baseline)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal([]byte(tags), &p.Tags); err != nil {
		return p, fmt.Errorf("profile %s tags: %w", p.ID, err)
	}
	if err := json.Unmarshal([]byte(fpDiff), &p.FPDiff); err != nil {
		return p, fmt.Errorf("profile %s fp_diff: %w", p.ID, err)
	}
	// An empty or broken value reads as "nothing pinned".
	_ = json.Unmarshal([]byte(meta), &p.EngineMeta)
	p.Machine = machineLabel(baseline)
	return p, nil
}

func tagsJSON(tags []string) string {
	if tags == nil {
		tags = []string{}
	}
	b, _ := json.Marshal(tags)
	return string(b)
}

// machineLabel turns a fingerprint baseline into "GPU · width×height". An
// ANGLE renderer string is cut to the GPU name after "Renderer: ".
func machineLabel(baselineJSON string) string {
	var b struct {
		WebGLRenderer string `json:"webglRenderer"`
		Screen        string `json:"screen"`
	}
	if json.Unmarshal([]byte(baselineJSON), &b) != nil {
		return ""
	}
	gpu := strings.TrimSpace(b.WebGLRenderer)
	if _, after, ok := strings.Cut(gpu, "Renderer: "); ok {
		gpu, _, _ = strings.Cut(after, ",")
		gpu = strings.TrimSuffix(strings.TrimSpace(gpu), ")")
	}
	if gpu == "" || b.Screen == "" {
		return ""
	}
	return gpu + " · " + strings.Replace(b.Screen, "x", "×", 1)
}

func metaJSON(m EngineMeta) string {
	b, _ := json.Marshal(m)
	return string(b)
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// StartURLs splits StartURL, which holds one URL per line.
func (p Profile) StartURLs() []string {
	var out []string
	for _, l := range strings.Split(p.StartURL, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func (s *Store) CreateProfile(p *Profile) error {
	if p.ID == "" || p.Name == "" || p.Engine == "" || p.PersonaID == "" || p.SeedHex == "" {
		return errors.New("profile needs id, name, engine, persona and seed")
	}
	if p.Site == "" {
		p.Site = "other"
	}
	p.CreatedAt, p.UpdatedAt = now(), now()
	_, err := s.db.Exec(`INSERT INTO profiles (id, name, engine, engine_version, persona_id, seed_hex,
		platform, proxy_id, timezone, languages, ui_locale, lat, lon, site, start_url, notes,
		tags, cdp_next_launch, pinned, created_at, updated_at, login, engine_meta, serial)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,(SELECT COALESCE(MAX(serial), 0) + 1 FROM profiles))`,
		p.ID, p.Name, p.Engine, p.EngineVersion, p.PersonaID, p.SeedHex, p.Platform, nullStr(p.ProxyID),
		p.Timezone, p.Languages, p.UILocale, p.Lat, p.Lon, p.Site, p.StartURL, p.Notes,
		tagsJSON(p.Tags), p.CDPNextLaunch, p.Pinned, p.CreatedAt, p.UpdatedAt, p.Login, metaJSON(p.EngineMeta))
	if err != nil {
		return err
	}
	return s.db.QueryRow(`SELECT serial FROM profiles WHERE id = ?`, p.ID).Scan(&p.Serial)
}

func (s *Store) GetProfile(id string) (Profile, error) {
	return scanProfile(s.db.QueryRow(`SELECT `+profileCols+` FROM profiles WHERE id = ?`, id))
}

// UpdateProfile saves the editable fields. It refuses to change the identity
// (engine, persona, seed, platform) fixed at creation.
func (s *Store) UpdateProfile(p Profile) error {
	cur, err := s.GetProfile(p.ID)
	if err != nil {
		return err
	}
	if p.Engine != cur.Engine || p.PersonaID != cur.PersonaID || p.SeedHex != cur.SeedHex || p.Platform != cur.Platform {
		return errors.New("engine, persona, seed and platform cannot change after creation")
	}
	_, err = s.db.Exec(`UPDATE profiles SET name=?, proxy_id=?, timezone=?, languages=?, ui_locale=?,
		lat=?, lon=?, site=?, start_url=?, notes=?, tags=?, cdp_next_launch=?, pinned=?, login=?,
		updated_at=? WHERE id=?`,
		p.Name, nullStr(p.ProxyID), p.Timezone, p.Languages, p.UILocale, p.Lat, p.Lon, p.Site,
		p.StartURL, p.Notes, tagsJSON(p.Tags), p.CDPNextLaunch, p.Pinned, p.Login, now(), p.ID)
	return err
}

// SetSecretFlag records whether a Keychain secret exists; flag is "has_password" or "has_totp".
func (s *Store) SetSecretFlag(id, flag string, on bool) error {
	if flag != "has_password" && flag != "has_totp" {
		return fmt.Errorf("unknown flag %q", flag)
	}
	res, err := s.db.Exec(`UPDATE profiles SET `+flag+` = ?, updated_at = ? WHERE id = ?`, on, now(), id)
	return affected(res, err)
}

// FingerprintBaseline returns the stored baseline sample (JSON), or "".
func (s *Store) FingerprintBaseline(id string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT fp_baseline FROM profiles WHERE id = ?`, id).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return v, err
}

// SaveFingerprint stores a measurement. An empty baseline keeps the old one;
// an empty last sample also keeps the old one (an unchecked run overwrites
// nothing).
func (s *Store) SaveFingerprint(id, baseline, last, verdict string, diff []string) error {
	if diff == nil {
		diff = []string{}
	}
	b, _ := json.Marshal(diff)
	_, err := s.db.Exec(`UPDATE profiles SET
		fp_baseline = CASE WHEN ? = '' THEN fp_baseline ELSE ? END,
		fp_last = CASE WHEN ? = '' THEN fp_last ELSE ? END,
		fp_verdict = ?, fp_diff = ?, fp_checked_at = ? WHERE id = ?`,
		baseline, baseline, last, last, verdict, string(b), now(), id)
	return err
}

// ResetFingerprint clears the baseline, last sample and verdict.
func (s *Store) ResetFingerprint(id string) error {
	res, err := s.db.Exec(`UPDATE profiles SET fp_baseline = '', fp_last = '', fp_verdict = '',
		fp_diff = '[]', fp_checked_at = '' WHERE id = ?`, id)
	return affected(res, err)
}

func (s *Store) ListProfiles(f ProfileFilter) ([]Profile, error) {
	where := []string{"deleted_at IS NULL"}
	if f.Trashed {
		where[0] = "deleted_at IS NOT NULL"
	}
	var args []any
	if q := strings.TrimSpace(f.Query); q != "" {
		where = append(where, `(name LIKE ? ESCAPE '\' OR login LIKE ? ESCAPE '\')`)
		args = append(args, "%"+likeEscape(q)+"%", "%"+likeEscape(q)+"%")
	}
	if f.Tag != "" {
		where = append(where, "EXISTS (SELECT 1 FROM json_each(profiles.tags) WHERE value = ?)")
		args = append(args, f.Tag)
	}
	rows, err := s.db.Query(`SELECT `+profileCols+` FROM profiles WHERE `+strings.Join(where, " AND ")+
		` ORDER BY pinned DESC, sort_order, created_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Profile{}
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (s *Store) SoftDelete(id string) error { return s.setDeleted(id, now()) }
func (s *Store) Restore(id string) error    { return s.setDeleted(id, nil) }

func (s *Store) setDeleted(id string, at any) error {
	res, err := s.db.Exec(`UPDATE profiles SET deleted_at = ?, updated_at = ? WHERE id = ?`, at, now(), id)
	return affected(res, err)
}

// Purge removes the row; the caller removes the profile directory and Keychain item.
func (s *Store) Purge(id string) error {
	res, err := s.db.Exec(`DELETE FROM profiles WHERE id = ?`, id)
	return affected(res, err)
}

func (s *Store) MarkOpened(id string) error {
	res, err := s.db.Exec(`UPDATE profiles SET last_open_at = ? WHERE id = ?`, now(), id)
	return affected(res, err)
}

func affected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
