// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"database/sql"
	"errors"
)

// Proxy is an upstream proxy. Its password is not here: it lives in the
// Keychain under secrets.ServiceProxy with the proxy id as the account.
type Proxy struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Scheme   string `json:"scheme"` // "http" | "socks5"
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	ExitIP   string `json:"exitIp"`
	Country  string `json:"country"`
	Timezone string `json:"timezone"`
	// ExitIP6 and Timezone6 are set when sites with IPv6 see the proxy at
	// another address than ExitIP.
	ExitIP6   string   `json:"exitIp6"`
	Timezone6 string   `json:"timezone6"`
	Lat       *float64 `json:"lat"`
	Lon       *float64 `json:"lon"`
	Rotating  bool     `json:"rotating"`
	CheckedAt string   `json:"checkedAt"`
	CreatedAt string   `json:"createdAt"`
	Note      string   `json:"note"`
	ExpiresOn string   `json:"expiresOn"` // YYYY-MM-DD, "" = no date
}

const proxyCols = `id, label, scheme, host, port, username, COALESCE(exit_ip, ''), COALESCE(country, ''),
	COALESCE(timezone, ''), lat, lon, rotating, COALESCE(checked_at, ''), created_at, note, expires_on, exit_ip6, timezone6`

func scanProxy(row interface{ Scan(...any) error }) (Proxy, error) {
	var p Proxy
	err := row.Scan(&p.ID, &p.Label, &p.Scheme, &p.Host, &p.Port, &p.Username, &p.ExitIP,
		&p.Country, &p.Timezone, &p.Lat, &p.Lon, &p.Rotating, &p.CheckedAt, &p.CreatedAt, &p.Note, &p.ExpiresOn,
		&p.ExitIP6, &p.Timezone6)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

func (s *Store) CreateProxy(p *Proxy) error {
	if p.ID == "" || p.Host == "" || p.Port <= 0 {
		return errors.New("proxy needs id, host and port")
	}
	p.CreatedAt = now()
	_, err := s.db.Exec(`INSERT INTO proxies (id, label, scheme, host, port, username, created_at, note, expires_on)
		VALUES (?,?,?,?,?,?,?,?,?)`, p.ID, p.Label, p.Scheme, p.Host, p.Port, p.Username, p.CreatedAt, p.Note, p.ExpiresOn)
	return err
}

func (s *Store) GetProxy(id string) (Proxy, error) {
	return scanProxy(s.db.QueryRow(`SELECT `+proxyCols+` FROM proxies WHERE id = ?`, id))
}

func (s *Store) ListProxies() ([]Proxy, error) {
	rows, err := s.db.Query(`SELECT ` + proxyCols + ` FROM proxies ORDER BY created_at, label`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Proxy{}
	for rows.Next() {
		p, err := scanProxy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpdateProxyMeta saves the editable proxy fields.
func (s *Store) UpdateProxyMeta(id, label, note, expiresOn string) error {
	res, err := s.db.Exec(`UPDATE proxies SET label = ?, note = ?, expires_on = ? WHERE id = ?`, label, note, expiresOn, id)
	return affected(res, err)
}

// SaveProxyCheck records where the proxy exits, as measured through it.
func (s *Store) SaveProxyCheck(p Proxy) error {
	res, err := s.db.Exec(`UPDATE proxies SET exit_ip=?, country=?, timezone=?, lat=?, lon=?, rotating=?,
		exit_ip6=?, timezone6=?, checked_at=? WHERE id=?`,
		p.ExitIP, p.Country, p.Timezone, p.Lat, p.Lon, p.Rotating, p.ExitIP6, p.Timezone6, now(), p.ID)
	return affected(res, err)
}

// DeleteProxy detaches it from profiles (ON DELETE SET NULL); the caller drops the Keychain item.
func (s *Store) DeleteProxy(id string) error {
	res, err := s.db.Exec(`DELETE FROM proxies WHERE id = ?`, id)
	return affected(res, err)
}

// ProxyUsage maps proxy id to the names of live profiles and mobile devices using it.
func (s *Store) ProxyUsage() (map[string][]string, error) {
	rows, err := s.db.Query(`SELECT proxy_id, name FROM profiles WHERE proxy_id IS NOT NULL AND deleted_at IS NULL
		UNION ALL SELECT proxy_id, name FROM mobile_devices WHERE proxy_id IS NOT NULL AND deleted_at IS NULL
		ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = append(out[id], name)
	}
	return out, rows.Err()
}
