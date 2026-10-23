// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// MobileDevice is one Android machine. Driver and Platform are fixed at
// creation; DriverRef is how the driver finds it (a folder, an LDPlayer name).
type MobileDevice struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Driver       string   `json:"driver"`
	Platform     string   `json:"platform"`
	DriverRef    string   `json:"driverRef"`
	ProxyID      string   `json:"proxyId"`
	RAMMB        int      `json:"ramMb"`
	CPUs         int      `json:"cpus"`
	ImageVersion string   `json:"imageVersion"`
	Notes        string   `json:"notes"`
	Tags         []string `json:"tags"`
	Pinned       bool     `json:"pinned"`
	Serial       int      `json:"serial"`
	CreatedAt    string   `json:"createdAt"`
	UpdatedAt    string   `json:"updatedAt"`
	LastOpenAt   string   `json:"lastOpenAt"`
	BackedUpAt   string   `json:"backedUpAt"`
	DeletedAt    string   `json:"deletedAt"`
}

// MobileLaunch records a running device. PIDStartTime guards against a recycled pid.
type MobileLaunch struct {
	DeviceID     string
	PID          int
	PIDStartTime string
	ADBPort      int
	RelayPort    int
	StartedAt    string
}

const mobileCols = `id, name, driver, platform, driver_ref, COALESCE(proxy_id, ''), ram_mb, cpus,
	image_version, notes, tags, pinned, serial, created_at, updated_at, COALESCE(last_open_at, ''),
	COALESCE(backed_up_at, ''), COALESCE(deleted_at, '')`

func scanMobile(row interface{ Scan(...any) error }) (MobileDevice, error) {
	var d MobileDevice
	var tags string
	err := row.Scan(&d.ID, &d.Name, &d.Driver, &d.Platform, &d.DriverRef, &d.ProxyID, &d.RAMMB, &d.CPUs,
		&d.ImageVersion, &d.Notes, &tags, &d.Pinned, &d.Serial, &d.CreatedAt, &d.UpdatedAt, &d.LastOpenAt,
		&d.BackedUpAt, &d.DeletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return d, ErrNotFound
	}
	if err != nil {
		return d, err
	}
	if err := json.Unmarshal([]byte(tags), &d.Tags); err != nil {
		return d, fmt.Errorf("mobile device %s tags: %w", d.ID, err)
	}
	return d, nil
}

func (s *Store) CreateMobileDevice(d *MobileDevice) error {
	if d.ID == "" || d.Name == "" || d.Driver == "" || d.Platform == "" {
		return errors.New("mobile device needs id, name, driver and platform")
	}
	d.CreatedAt, d.UpdatedAt = now(), now()
	_, err := s.db.Exec(`INSERT INTO mobile_devices (id, name, driver, platform, driver_ref, proxy_id, ram_mb,
		cpus, image_version, notes, tags, pinned, created_at, updated_at, serial)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,(SELECT COALESCE(MAX(serial), 0) + 1 FROM mobile_devices))`,
		d.ID, d.Name, d.Driver, d.Platform, d.DriverRef, nullStr(d.ProxyID), d.RAMMB, d.CPUs, d.ImageVersion,
		d.Notes, tagsJSON(d.Tags), d.Pinned, d.CreatedAt, d.UpdatedAt)
	if err != nil {
		return err
	}
	return s.db.QueryRow(`SELECT serial FROM mobile_devices WHERE id = ?`, d.ID).Scan(&d.Serial)
}

func (s *Store) GetMobileDevice(id string) (MobileDevice, error) {
	return scanMobile(s.db.QueryRow(`SELECT `+mobileCols+` FROM mobile_devices WHERE id = ?`, id))
}

// UpdateMobileDevice saves the editable fields; driver and platform never change.
func (s *Store) UpdateMobileDevice(d MobileDevice) error {
	res, err := s.db.Exec(`UPDATE mobile_devices SET name=?, proxy_id=?, ram_mb=?, cpus=?, notes=?, tags=?,
		pinned=?, updated_at=? WHERE id=?`,
		d.Name, nullStr(d.ProxyID), d.RAMMB, d.CPUs, d.Notes, tagsJSON(d.Tags), d.Pinned, now(), d.ID)
	return affected(res, err)
}

// SetMobileDriverState records what the driver made at creation.
func (s *Store) SetMobileDriverState(id, ref, imageVersion string) error {
	res, err := s.db.Exec(`UPDATE mobile_devices SET driver_ref=?, image_version=?, updated_at=? WHERE id=?`,
		ref, imageVersion, now(), id)
	return affected(res, err)
}

// ListMobileDevices lists live devices, or the trash, filtered like ListProfiles.
func (s *Store) ListMobileDevices(f ProfileFilter) ([]MobileDevice, error) {
	where := []string{"deleted_at IS NULL"}
	if f.Trashed {
		where[0] = "deleted_at IS NOT NULL"
	}
	var args []any
	if q := strings.TrimSpace(f.Query); q != "" {
		if n, err := strconv.Atoi(strings.TrimPrefix(q, "#")); err == nil && strings.HasPrefix(q, "#") {
			where = append(where, "serial = ?")
			args = append(args, n)
		} else {
			like := "%" + likeEscape(q) + "%"
			where = append(where, `(name LIKE ? ESCAPE '\' OR notes LIKE ? ESCAPE '\' OR tags LIKE ? ESCAPE '\')`)
			args = append(args, like, like, like)
		}
	}
	if f.Tag != "" {
		where = append(where, "EXISTS (SELECT 1 FROM json_each(mobile_devices.tags) WHERE value = ?)")
		args = append(args, f.Tag)
	}
	rows, err := s.db.Query(`SELECT `+mobileCols+` FROM mobile_devices WHERE `+strings.Join(where, " AND ")+
		` ORDER BY pinned DESC, sort_order, created_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MobileDevice{}
	for rows.Next() {
		d, err := scanMobile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) TrashMobileDevice(id string) error   { return s.setMobileDeleted(id, now()) }
func (s *Store) RestoreMobileDevice(id string) error { return s.setMobileDeleted(id, nil) }

func (s *Store) setMobileDeleted(id string, at any) error {
	res, err := s.db.Exec(`UPDATE mobile_devices SET deleted_at = ?, updated_at = ? WHERE id = ?`, at, now(), id)
	return affected(res, err)
}

// PurgeMobileDevice removes the row; the caller removes the device itself first.
func (s *Store) PurgeMobileDevice(id string) error {
	res, err := s.db.Exec(`DELETE FROM mobile_devices WHERE id = ?`, id)
	return affected(res, err)
}

func (s *Store) MarkMobileOpened(id string) error {
	res, err := s.db.Exec(`UPDATE mobile_devices SET last_open_at = ? WHERE id = ?`, now(), id)
	return affected(res, err)
}

func (s *Store) MarkMobileBackedUp(id string) error {
	res, err := s.db.Exec(`UPDATE mobile_devices SET backed_up_at = ? WHERE id = ?`, now(), id)
	return affected(res, err)
}

func (s *Store) PutMobileLaunch(l MobileLaunch) error {
	_, err := s.db.Exec(`INSERT INTO mobile_launches (device_id, pid, pid_start_time, adb_port, relay_port, started_at)
		VALUES (?,?,?,?,?,?) ON CONFLICT(device_id) DO UPDATE SET pid=excluded.pid,
		pid_start_time=excluded.pid_start_time, adb_port=excluded.adb_port, relay_port=excluded.relay_port,
		started_at=excluded.started_at`,
		l.DeviceID, l.PID, l.PIDStartTime, l.ADBPort, l.RelayPort, now())
	return err
}

func (s *Store) DeleteMobileLaunch(deviceID string) error {
	_, err := s.db.Exec(`DELETE FROM mobile_launches WHERE device_id = ?`, deviceID)
	return err
}

func (s *Store) ListMobileLaunches() ([]MobileLaunch, error) {
	rows, err := s.db.Query(`SELECT device_id, pid, pid_start_time, adb_port, relay_port, started_at FROM mobile_launches`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MobileLaunch
	for rows.Next() {
		var l MobileLaunch
		if err := rows.Scan(&l.DeviceID, &l.PID, &l.PIDStartTime, &l.ADBPort, &l.RelayPort, &l.StartedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
