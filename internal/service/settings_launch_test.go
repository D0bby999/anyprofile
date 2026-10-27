// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"strings"
	"testing"

	"github.com/D0bby999/anyprofile/internal/store"
)

func TestMaxConcurrentLaunches(t *testing.T) {
	s, _ := newService(t)
	if got := s.MaxConcurrentLaunches(); got != 0 {
		t.Fatalf("default = %d, want 0 (unlimited)", got)
	}
	if err := s.SetMaxConcurrentLaunches(4); err != nil {
		t.Fatal(err)
	}
	if got := s.MaxConcurrentLaunches(); got != 4 {
		t.Fatalf("after set = %d, want 4", got)
	}
	if got := s.Launcher.MaxRunning(); got != 4 {
		t.Fatalf("launcher ceiling = %d, want 4 (set must apply at once)", got)
	}
	err := s.SetMaxConcurrentLaunches(-1)
	if err == nil || !strings.Contains(err.Error(), "bad_max_concurrent_launches") {
		t.Fatalf("SetMaxConcurrentLaunches(-1) err = %v", err)
	}
	if got := s.MaxConcurrentLaunches(); got != 4 {
		t.Fatalf("after refused set = %d, want 4 (unchanged)", got)
	}
}

func TestMaxConcurrentLaunchesBadStoredValue(t *testing.T) {
	s, _ := newService(t)
	if err := s.Store.SetSetting(store.SettingMaxConcurrentLaunches, "many"); err != nil {
		t.Fatal(err)
	}
	if got := s.MaxConcurrentLaunches(); got != 0 {
		t.Fatalf("unreadable stored value = %d, want 0 (unlimited)", got)
	}
}

func TestApplyLaunchCeiling(t *testing.T) {
	s, _ := newService(t)
	if err := s.Store.SetSetting(store.SettingMaxConcurrentLaunches, "2"); err != nil {
		t.Fatal(err)
	}
	if got := s.Launcher.MaxRunning(); got != 0 {
		t.Fatalf("launcher ceiling before apply = %d, want 0", got)
	}
	s.ApplyLaunchCeiling()
	if got := s.Launcher.MaxRunning(); got != 2 {
		t.Fatalf("launcher ceiling after apply = %d, want 2", got)
	}
}
