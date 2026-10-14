// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

// MaxSessions is how many sessions a profile keeps; older ones are dropped.
const MaxSessions = 50

// Session is one browser launch of a profile.
type Session struct {
	ID            int64           `json:"id"`
	ProfileID     string          `json:"profileId"`
	Engine        string          `json:"engine"`
	EngineVersion string          `json:"engineVersion"`
	ProxyID       string          `json:"proxyId"`
	StartedAt     string          `json:"startedAt"`
	StoppedAt     string          `json:"stoppedAt"`
	ExitIP        string          `json:"exitIp"`
	ExitCountry   string          `json:"exitCountry"`
	ExitTZ        string          `json:"exitTz"`
	ExitError     string          `json:"exitError"`
	Checks        json.RawMessage `json:"checks"`
}

const sessionCols = `id, profile_id, engine, engine_version, proxy_id, started_at, stopped_at,
	exit_ip, exit_country, exit_tz, exit_error, checks`

func scanSession(row interface{ Scan(...any) error }) (Session, error) {
	var x Session
	var checks string
	err := row.Scan(&x.ID, &x.ProfileID, &x.Engine, &x.EngineVersion, &x.ProxyID, &x.StartedAt, &x.StoppedAt,
		&x.ExitIP, &x.ExitCountry, &x.ExitTZ, &x.ExitError, &checks)
	x.Checks = json.RawMessage(checks)
	return x, err
}

// StartSession records a launch and keeps only the newest MaxSessions of the profile.
func (s *Store) StartSession(x *Session) error {
	x.StartedAt = now()
	res, err := s.db.Exec(`INSERT INTO sessions (profile_id, engine, engine_version, proxy_id, started_at)
		VALUES (?, ?, ?, ?, ?)`, x.ProfileID, x.Engine, x.EngineVersion, x.ProxyID, x.StartedAt)
	if err != nil {
		return err
	}
	if x.ID, err = res.LastInsertId(); err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM sessions WHERE profile_id = ? AND id NOT IN
		(SELECT id FROM sessions WHERE profile_id = ? ORDER BY id DESC LIMIT ?)`, x.ProfileID, x.ProfileID, MaxSessions)
	return err
}

// EndSession stamps the stop time once; a second call keeps the first.
func (s *Store) EndSession(id int64) error {
	_, err := s.db.Exec(`UPDATE sessions SET stopped_at = ? WHERE id = ? AND stopped_at = ''`, now(), id)
	return err
}

func (s *Store) SetSessionExit(id int64, ip, country, tz, errMsg string) error {
	_, err := s.db.Exec(`UPDATE sessions SET exit_ip = ?, exit_country = ?, exit_tz = ?, exit_error = ? WHERE id = ?`,
		ip, country, tz, errMsg, id)
	return err
}

func (s *Store) SetSessionChecks(id int64, checks []byte) error {
	_, err := s.db.Exec(`UPDATE sessions SET checks = ? WHERE id = ?`, string(checks), id)
	return err
}

// OpenSession is the newest session of the profile that has not stopped.
func (s *Store) OpenSession(profileID string) (Session, bool, error) {
	x, err := scanSession(s.db.QueryRow(`SELECT `+sessionCols+` FROM sessions
		WHERE profile_id = ? AND stopped_at = '' ORDER BY id DESC LIMIT 1`, profileID))
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, false, nil
	}
	return x, err == nil, err
}

// Sessions lists the profile's sessions, newest first.
func (s *Store) Sessions(profileID string, limit int) ([]Session, error) {
	rows, err := s.db.Query(`SELECT `+sessionCols+` FROM sessions WHERE profile_id = ? ORDER BY id DESC LIMIT ?`, profileID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		x, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// LatestExits gives, per profile, its two newest sessions with a measured
// exit or a measuring error: [0] is the newest, [1] the one before (zero ID
// when there is none).
func (s *Store) LatestExits(profileIDs []string) (map[string][2]Session, error) {
	out := map[string][2]Session{}
	if len(profileIDs) == 0 {
		return out, nil
	}
	args := make([]any, len(profileIDs))
	for i, id := range profileIDs {
		args[i] = id
	}
	rows, err := s.db.Query(`SELECT `+sessionCols+`, rn FROM (
		SELECT *, ROW_NUMBER() OVER (PARTITION BY profile_id ORDER BY id DESC) AS rn FROM sessions
		WHERE (exit_ip <> '' OR exit_error <> '') AND profile_id IN (?`+strings.Repeat(",?", len(args)-1)+`)
	) WHERE rn <= 2`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var x Session
		var checks string
		var rn int
		if err := rows.Scan(&x.ID, &x.ProfileID, &x.Engine, &x.EngineVersion, &x.ProxyID, &x.StartedAt, &x.StoppedAt,
			&x.ExitIP, &x.ExitCountry, &x.ExitTZ, &x.ExitError, &checks, &rn); err != nil {
			return nil, err
		}
		x.Checks = json.RawMessage(checks)
		pair := out[x.ProfileID]
		pair[rn-1] = x
		out[x.ProfileID] = pair
	}
	return out, rows.Err()
}

// CloseStaleSessions stamps sessions left open by profiles that are not running
// (the app quit or crashed while they ran).
func (s *Store) CloseStaleSessions(running map[string]bool) error {
	rows, err := s.db.Query(`SELECT id, profile_id FROM sessions WHERE stopped_at = ''`)
	if err != nil {
		return err
	}
	var stale []int64
	for rows.Next() {
		var id int64
		var pid string
		if err := rows.Scan(&id, &pid); err != nil {
			rows.Close()
			return err
		}
		if !running[pid] {
			stale = append(stale, id)
		}
	}
	rows.Close()
	for _, id := range stale {
		if err := s.EndSession(id); err != nil {
			return err
		}
	}
	return nil
}

// LatestChecks gives, per profile, the checks of its newest session that has any.
func (s *Store) LatestChecks(profileIDs []string) (map[string]json.RawMessage, error) {
	out := map[string]json.RawMessage{}
	if len(profileIDs) == 0 {
		return out, nil
	}
	args := make([]any, len(profileIDs))
	for i, id := range profileIDs {
		args[i] = id
	}
	rows, err := s.db.Query(`SELECT profile_id, checks FROM (
		SELECT profile_id, checks, ROW_NUMBER() OVER (PARTITION BY profile_id ORDER BY id DESC) AS rn FROM sessions
		WHERE checks <> '[]' AND profile_id IN (?`+strings.Repeat(",?", len(args)-1)+`)
	) WHERE rn = 1`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, checks string
		if err := rows.Scan(&id, &checks); err != nil {
			return nil, err
		}
		out[id] = json.RawMessage(checks)
	}
	return out, rows.Err()
}
