// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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
