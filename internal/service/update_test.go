// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/D0bby999/anyprofile/internal/update"
)

func TestCheckUpdateOncePerDayAndOnDemand(t *testing.T) {
	s, _ := newService(t)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		json.NewEncoder(w).Encode(update.Manifest{Version: "99.0.0", Date: "2026-10-16", Notes: "Faster."})
	}))
	defer srv.Close()
	t.Setenv(update.EnvManifestURL, srv.URL)
	ctx := context.Background()

	if st := s.UpdateStatus(); !st.Auto || st.Available || st.Current != update.Current {
		t.Fatalf("fresh status = %+v", st)
	}
	st, err := s.CheckUpdate(ctx, false)
	if err != nil || !st.Available || st.Latest != "99.0.0" || st.Notes != "Faster." || st.CheckedAt == "" {
		t.Fatalf("auto check = %+v, %v", st, err)
	}
	// A second automatic check the same day does not ask the server again.
	if _, err := s.CheckUpdate(ctx, false); err != nil || hits.Load() != 1 {
		t.Fatalf("second auto check: hits %d, %v", hits.Load(), err)
	}
	// The button always asks.
	if _, err := s.CheckUpdate(ctx, true); err != nil || hits.Load() != 2 {
		t.Fatalf("forced check: hits %d, %v", hits.Load(), err)
	}
	// Automatic checks off: nothing is asked even when a day has passed.
	if err := s.SetUpdateAuto(false); err != nil {
		t.Fatal(err)
	}
	s.Store.SetSetting("update_checked_at", "2000-01-01T00:00:00Z")
	if st, _ := s.CheckUpdate(ctx, false); st.Auto || hits.Load() != 2 {
		t.Fatalf("auto off: %+v, hits %d", st, hits.Load())
	}
}

func TestCheckUpdateFailures(t *testing.T) {
	s, _ := newService(t)
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	t.Setenv(update.EnvManifestURL, srv.URL)
	if _, err := s.CheckUpdate(context.Background(), false); err != nil {
		t.Fatalf("a failed automatic check must stay quiet: %v", err)
	}
	if _, err := s.CheckUpdate(context.Background(), true); err == nil {
		t.Fatal("a failed forced check must report")
	}
}

func TestInstallUpdateWhenAlreadyNewest(t *testing.T) {
	s, _ := newService(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(update.Manifest{Version: update.Current})
	}))
	defer srv.Close()
	t.Setenv(update.EnvManifestURL, srv.URL)
	if err := s.InstallUpdate(context.Background(), nil); err == nil {
		t.Fatal("installed the running version")
	}
	if _, err := os.Stat(s.updatesDir()); err == nil {
		t.Fatal("downloaded anything for the running version")
	}
}

func TestReadyUpdateAndCleanUp(t *testing.T) {
	s, _ := newService(t)
	s.Store.SetSetting("update_latest", `{"version":"99.0.0"}`)
	pkg := filepath.Join(s.updatesDir(), "AnyProfile.zip")
	os.MkdirAll(s.updatesDir(), 0o700)
	os.WriteFile(pkg, []byte("x"), 0o600)
	ready, _ := json.Marshal(readyUpdate{Version: "99.0.0", Path: pkg}) // JSON-escaped: a Windows path has backslashes
	s.Store.SetSetting("update_ready", string(ready))
	if st := s.UpdateStatus(); !st.Available || !st.Ready {
		t.Fatalf("status = %+v", st)
	}
	// A ready release newer than this one survives the clean-up at launch.
	s.CleanUpdates()
	if _, err := os.Stat(pkg); err != nil {
		t.Fatal("the ready update was removed")
	}
	// Once it is the running version (or older), it goes.
	s.Store.SetSetting("update_ready", `{"version":"`+update.Current+`","path":"`+pkg+`"}`)
	s.CleanUpdates()
	if _, err := os.Stat(pkg); !os.IsNotExist(err) {
		t.Fatal("an installed update was kept")
	}
	if st := s.UpdateStatus(); st.Ready {
		t.Fatalf("ready without a file: %+v", st)
	}
}

func TestPrefetchUpdateKeepsNothingUnverified(t *testing.T) {
	s, _ := newService(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(update.Manifest{Version: "99.0.0"}) // no package for this computer
	}))
	defer srv.Close()
	t.Setenv(update.EnvManifestURL, srv.URL)
	if _, err := s.CheckUpdate(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if err := s.PrefetchUpdate(context.Background()); err == nil {
		t.Fatal("want an error for a release without a package")
	}
	if st := s.UpdateStatus(); !st.Available || st.Ready {
		t.Fatalf("status = %+v", st)
	}
}

func TestWhatsNewOnceAfterAnUpdate(t *testing.T) {
	s, _ := newService(t)
	// First install: nothing to show, the version is remembered.
	if w := s.TakeWhatsNew(); w.Version != "" {
		t.Fatalf("first run = %+v", w)
	}
	// An update to the release whose notes were fetched before.
	s.Store.SetSetting("last_version", "0.0.1")
	s.Store.SetSetting("update_latest", `{"version":"`+update.Current+`","notes":"Backups."}`)
	if w := s.TakeWhatsNew(); w.Version != update.Current || w.Notes != "Backups." {
		t.Fatalf("after update = %+v", w)
	}
	if w := s.TakeWhatsNew(); w.Version != "" {
		t.Fatalf("shown twice: %+v", w)
	}
}
