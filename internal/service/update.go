// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/update"
)

// UpdateStatus is what Settings and the status bar show about new releases.
type UpdateStatus struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"` // "" until a check found a release
	Date      string `json:"date"`
	Notes     string `json:"notes"`
	Available bool   `json:"available"`
	CheckedAt string `json:"checkedAt"` // RFC 3339, "" when never checked
	Auto      bool   `json:"auto"`      // check once a day on its own
	Ready     bool   `json:"ready"`     // Latest is downloaded and verified: installing is a restart
}

// WhatsNew is the release notes of the version just installed, shown once.
type WhatsNew struct {
	Version string `json:"version"`
	Notes   string `json:"notes"`
}

// readyUpdate is a downloaded, verified update waiting to be installed.
type readyUpdate struct {
	Version string `json:"version"`
	Path    string `json:"path"`
}

// updateEvery is how often an automatic check runs.
const updateEvery = 24 * time.Hour

type knownRelease struct {
	Version string `json:"version"`
	Date    string `json:"date"`
	Notes   string `json:"notes"`
}

func (s *Service) UpdateStatus() UpdateStatus {
	st := UpdateStatus{Current: update.Current, Auto: true}
	if v, _ := s.Store.GetSetting("update_auto"); v == "0" {
		st.Auto = false
	}
	st.CheckedAt, _ = s.Store.GetSetting("update_checked_at")
	var k knownRelease
	if v, _ := s.Store.GetSetting("update_latest"); json.Unmarshal([]byte(v), &k) == nil {
		st.Latest, st.Date, st.Notes = k.Version, k.Date, k.Notes
		st.Available = update.Newer(k.Version, update.Current)
	}
	if r, ok := s.readyUpdate(); ok && st.Available && r.Version == st.Latest {
		st.Ready = true
	}
	return st
}

func (s *Service) readyUpdate() (readyUpdate, bool) {
	var r readyUpdate
	v, _ := s.Store.GetSetting("update_ready")
	if json.Unmarshal([]byte(v), &r) != nil || r.Path == "" || !fileExists(r.Path) {
		return r, false
	}
	return r, true
}

// PrefetchUpdate downloads and verifies a newer release in the background,
// so that installing it later is only a restart. It does nothing when one
// is ready already or none is newer.
func (s *Service) PrefetchUpdate(ctx context.Context) error {
	s.updMu.Lock()
	defer s.updMu.Unlock()
	st := s.UpdateStatus()
	if !st.Available || st.Ready {
		return nil
	}
	_, err := s.download(ctx, nil)
	return err
}

// download fetches latest.json and the newest release's package, verified,
// and records it as ready. Callers hold updMu.
func (s *Service) download(ctx context.Context, progress update.Progress) (string, error) {
	fctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	m, err := update.Fetch(fctx, http.DefaultClient)
	cancel()
	if err != nil {
		return "", apperr.Newf("update_check", "could not check for updates: %v", err)
	}
	if !update.Newer(m.Version, update.Current) {
		return "", apperr.Newf("up_to_date", "AnyProfile %s is the newest version", update.Current)
	}
	a, err := m.AssetFor(update.Platform())
	if err != nil {
		return "", apperr.Newf("update_bad", "%v", err)
	}
	if r, ok := s.readyUpdate(); ok && r.Version == m.Version && update.VerifyFile(r.Path, a) == nil {
		return r.Path, nil
	}
	dctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	dir := s.updatesDir()
	os.RemoveAll(dir)
	pkg, err := update.Download(dctx, http.DefaultClient, a, dir, progress)
	if err != nil {
		return "", apperr.Newf("update_download", "%v", err)
	}
	b, _ := json.Marshal(readyUpdate{Version: m.Version, Path: pkg})
	if err := s.Store.SetSetting("update_ready", string(b)); err != nil {
		return "", err
	}
	s.Log.Info("update downloaded", "version", m.Version)
	s.notify("update")
	return pkg, nil
}

// TakeWhatsNew returns the notes of the version that runs now, the first
// time it runs after an update; otherwise an empty WhatsNew. A first install
// shows nothing.
func (s *Service) TakeWhatsNew() WhatsNew {
	last, _ := s.Store.GetSetting("last_version")
	if last == update.Current {
		return WhatsNew{}
	}
	if err := s.Store.SetSetting("last_version", update.Current); err != nil {
		return WhatsNew{}
	}
	var k knownRelease
	v, _ := s.Store.GetSetting("update_latest")
	if last == "" || json.Unmarshal([]byte(v), &k) != nil || k.Version != update.Current || k.Notes == "" {
		return WhatsNew{}
	}
	return WhatsNew{Version: k.Version, Notes: k.Notes}
}

func (s *Service) SetUpdateAuto(on bool) error {
	v := "1"
	if !on {
		v = "0"
	}
	return s.Store.SetSetting("update_auto", v)
}

// CheckUpdate asks the release server for the newest version. Without force
// it only does so when automatic checks are on and the last one is a day old,
// and a failure is logged, not returned: nobody asked.
func (s *Service) CheckUpdate(ctx context.Context, force bool) (UpdateStatus, error) {
	st := s.UpdateStatus()
	if !force {
		last, err := time.Parse(time.RFC3339, st.CheckedAt)
		if !st.Auto || (err == nil && time.Since(last) < updateEvery) {
			return st, nil
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	m, err := update.Fetch(ctx, http.DefaultClient)
	if err != nil {
		if !force {
			s.Log.Warn("update check", "err", err)
			return st, nil
		}
		return st, apperr.Newf("update_check", "could not check for updates: %v", err)
	}
	s.takeEngineList(m.Engines, m.EnginesSig)
	b, _ := json.Marshal(knownRelease{Version: m.Version, Date: m.Date, Notes: m.Notes})
	if err := s.Store.SetSetting("update_latest", string(b)); err != nil {
		return st, err
	}
	if err := s.Store.SetSetting("update_checked_at", time.Now().UTC().Format(time.RFC3339)); err != nil {
		return st, err
	}
	return s.UpdateStatus(), nil
}

// InstallUpdate downloads and verifies the newest release, and hands it to
// the installer, which starts the new version once this one has quit. The
// caller quits the app when it returns nil. Open profiles must be closed
// first: the installer replaces the app that manages them.
func (s *Service) InstallUpdate(ctx context.Context, progress update.Progress) error {
	if n := len(s.Launcher.List()); n > 0 {
		return apperr.Newf("profiles_open", "close the %d open profiles before updating", n)
	}
	s.updMu.Lock() // waits for a download in the background, then uses it
	defer s.updMu.Unlock()
	pkg, err := s.download(ctx, progress)
	if err != nil {
		return err
	}
	s.Log.Info("installing update", "from", update.Current)
	if err := update.Apply(pkg, os.Getpid()); err != nil {
		return apperr.Newf("update_install", "%v", err)
	}
	return nil
}

func (s *Service) updatesDir() string { return filepath.Join(s.Dirs.Root, "updates") }

// CleanUpdates removes downloads and swapped-out copies an earlier update
// left behind. A downloaded release newer than this one stays, ready.
func (s *Service) CleanUpdates() {
	if r, ok := s.readyUpdate(); !ok || !update.Newer(r.Version, update.Current) {
		os.RemoveAll(s.updatesDir())
		s.Store.SetSetting("update_ready", "")
	}
	update.CleanStale()
}

// engineList is the last verified engine list, kept so it holds offline.
type engineList struct {
	Raw json.RawMessage `json:"raw"`
	Sig string          `json:"sig"`
}

// takeEngineList verifies the engine list of a fetched manifest and, when it
// passes, keeps it and adds it to the catalog. A bad list is logged and the
// one kept before stays.
func (s *Service) takeEngineList(raw json.RawMessage, sig string) {
	if s.Catalog == nil || len(raw) == 0 {
		return
	}
	rs, err := update.VerifyEngines(raw, sig)
	if err != nil {
		s.Log.Warn("engine list", "err", err)
		return
	}
	if err := s.Catalog.SetListed(rs); err != nil {
		s.Log.Warn("engine list has releases that cannot be used", "err", err)
	}
	b, _ := json.Marshal(engineList{Raw: raw, Sig: sig})
	if err := s.Store.SetSetting("engine_list", string(b)); err != nil {
		s.Log.Warn("save engine list", "err", err)
	}
	s.notify("engines")
}

// LoadEngineList adds the kept engine list to the catalog, checking its
// signature again: the database is not trusted with it.
func (s *Service) LoadEngineList() {
	v, _ := s.Store.GetSetting("engine_list")
	var l engineList
	if s.Catalog == nil || v == "" || json.Unmarshal([]byte(v), &l) != nil {
		return
	}
	rs, err := update.VerifyEngines(l.Raw, l.Sig)
	if err != nil {
		s.Log.Warn("kept engine list", "err", err)
		return
	}
	if err := s.Catalog.SetListed(rs); err != nil {
		s.Log.Warn("kept engine list has releases that cannot be used", "err", err)
	}
}
