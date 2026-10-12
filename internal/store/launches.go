// SPDX-License-Identifier: AGPL-3.0-or-later

package store

// Launch records a running browser so the app can adopt it after a restart.
// PIDStartTime guards against a recycled pid.
type Launch struct {
	ProfileID    string
	PID          int
	PIDStartTime string
	RelayPort    int
	CDPPort      int
	StartedAt    string
}

func (s *Store) PutLaunch(l Launch) error {
	_, err := s.db.Exec(`INSERT INTO launches (profile_id, pid, pid_start_time, relay_port, cdp_port, started_at)
		VALUES (?,?,?,?,?,?) ON CONFLICT(profile_id) DO UPDATE SET pid=excluded.pid,
		pid_start_time=excluded.pid_start_time, relay_port=excluded.relay_port,
		cdp_port=excluded.cdp_port, started_at=excluded.started_at`,
		l.ProfileID, l.PID, l.PIDStartTime, l.RelayPort, l.CDPPort, now())
	return err
}

func (s *Store) DeleteLaunch(profileID string) error {
	_, err := s.db.Exec(`DELETE FROM launches WHERE profile_id = ?`, profileID)
	return err
}

func (s *Store) ListLaunches() ([]Launch, error) {
	rows, err := s.db.Query(`SELECT profile_id, pid, pid_start_time, relay_port, cdp_port, started_at FROM launches`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Launch
	for rows.Next() {
		var l Launch
		if err := rows.Scan(&l.ProfileID, &l.PID, &l.PIDStartTime, &l.RelayPort, &l.CDPPort, &l.StartedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
