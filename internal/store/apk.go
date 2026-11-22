// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// MobileAPK is one standalone Android app added through AnyProfile's Pro APK tab.
type MobileAPK struct {
	Package        string   `json:"package"`
	Label          string   `json:"label"`
	VersionName    string   `json:"versionName"`
	VersionCode    int32    `json:"versionCode"`
	LaunchActivity string   `json:"launchActivity"`
	Orientation    string   `json:"orientation"`
	ABIs           []string `json:"abis"`
	FilePath       string   `json:"filePath"`
	IconData       string   `json:"iconData"`
	DeviceID       string   `json:"deviceId"`
	ProxyID        string   `json:"proxyId"`
	Notes          string   `json:"notes"`
	IdentityJSON   string   `json:"identityJson"`
	AndroidID      string   `json:"androidId"`
	ShortcutPath   string   `json:"shortcutPath"`
	InstalledAt    string   `json:"installedAt"`
	LastOpenAt     string   `json:"lastOpenAt"`
}

const apkCols = `package, label, version_name, version_code, launch_activity, orientation, abis_json, file_path, icon_data, device_id, proxy_id, notes, identity_json, android_id, shortcut_path, installed_at, last_open_at`

func scanAPK(row interface{ Scan(...any) error }) (MobileAPK, error) {
	var a MobileAPK
	var abisJSON string
	err := row.Scan(&a.Package, &a.Label, &a.VersionName, &a.VersionCode, &a.LaunchActivity, &a.Orientation,
		&abisJSON, &a.FilePath, &a.IconData, &a.DeviceID, &a.ProxyID, &a.Notes, &a.IdentityJSON, &a.AndroidID,
		&a.ShortcutPath, &a.InstalledAt, &a.LastOpenAt)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	if err != nil {
		return a, err
	}
	_ = json.Unmarshal([]byte(abisJSON), &a.ABIs)
	if a.ABIs == nil {
		a.ABIs = []string{}
	}
	return a, nil
}

func (s *Store) ListMobileAPKs() ([]MobileAPK, error) {
	rows, err := s.db.Query(`SELECT ` + apkCols + ` FROM mobile_apks ORDER BY last_open_at DESC, label ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MobileAPK, 0)
	for rows.Next() {
		a, err := scanAPK(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) GetMobileAPK(pkg string) (MobileAPK, error) {
	return scanAPK(s.db.QueryRow(`SELECT `+apkCols+` FROM mobile_apks WHERE package = ?`, pkg))
}

func (s *Store) UpsertMobileAPK(a *MobileAPK) error {
	if a.Package == "" || a.Label == "" {
		return errors.New("mobile apk needs package and label")
	}
	if a.Orientation == "" {
		a.Orientation = "auto"
	}
	if a.VersionName == "" {
		a.VersionName = "1.0"
	}
	if a.InstalledAt == "" {
		a.InstalledAt = time.Now().UTC().Format(time.RFC3339)
	}
	abisJSON, _ := json.Marshal(a.ABIs)
	_, err := s.db.Exec(`
		INSERT INTO mobile_apks (`+apkCols+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(package) DO UPDATE SET
			label = excluded.label,
			version_name = excluded.version_name,
			version_code = excluded.version_code,
			launch_activity = excluded.launch_activity,
			orientation = excluded.orientation,
			abis_json = excluded.abis_json,
			file_path = CASE WHEN excluded.file_path != '' THEN excluded.file_path ELSE mobile_apks.file_path END,
			icon_data = CASE WHEN excluded.icon_data != '' THEN excluded.icon_data ELSE mobile_apks.icon_data END,
			device_id = excluded.device_id,
			proxy_id = excluded.proxy_id,
			notes = excluded.notes,
			identity_json = CASE WHEN excluded.identity_json != '' THEN excluded.identity_json ELSE mobile_apks.identity_json END,
			android_id = CASE WHEN excluded.android_id != '' THEN excluded.android_id ELSE mobile_apks.android_id END,
			shortcut_path = CASE WHEN excluded.shortcut_path != '' THEN excluded.shortcut_path ELSE mobile_apks.shortcut_path END
	`, a.Package, a.Label, a.VersionName, a.VersionCode, a.LaunchActivity, a.Orientation, string(abisJSON),
		a.FilePath, a.IconData, a.DeviceID, a.ProxyID, a.Notes, a.IdentityJSON, a.AndroidID,
		a.ShortcutPath, a.InstalledAt, a.LastOpenAt)
	return err
}

func (s *Store) UpdateMobileAPK(pkg, label, orientation, proxyID, notes, identityJSON, androidID string) error {
	res, err := s.db.Exec(
		`UPDATE mobile_apks SET label = ?, orientation = ?, proxy_id = ?, notes = ?, identity_json = ?, android_id = ? WHERE package = ?`,
		label, orientation, proxyID, notes, identityJSON, androidID, pkg,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetMobileAPKIdentity(pkg, identityJSON, androidID string) error {
	res, err := s.db.Exec(`UPDATE mobile_apks SET identity_json = ?, android_id = ? WHERE package = ?`, identityJSON, androidID, pkg)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetMobileAPKOrientation(pkg, orientation string) error {
	res, err := s.db.Exec(`UPDATE mobile_apks SET orientation = ? WHERE package = ?`, orientation, pkg)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetMobileAPKDevice(pkg, deviceID string) error {
	res, err := s.db.Exec(`UPDATE mobile_apks SET device_id = ? WHERE package = ?`, deviceID, pkg)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetMobileAPKShortcut(pkg, path string) error {
	res, err := s.db.Exec(`UPDATE mobile_apks SET shortcut_path = ? WHERE package = ?`, path, pkg)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) TouchMobileAPKOpened(pkg string) error {
	_, err := s.db.Exec(`UPDATE mobile_apks SET last_open_at = ? WHERE package = ?`, time.Now().UTC().Format(time.RFC3339), pkg)
	return err
}

func (s *Store) DeleteMobileAPK(pkg string) error {
	_, err := s.db.Exec(`DELETE FROM mobile_apks WHERE package = ?`, pkg)
	return err
}
