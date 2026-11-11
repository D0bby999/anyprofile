// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/service"
	"github.com/D0bby999/anyprofile/internal/store"
)

func TestFormatTrayState(t *testing.T) {
	cases := []struct {
		name        string
		lang        string
		browsers    int
		mobile      int
		apiRunning  bool
		wantActive  bool
		wantRunning bool
		wantStatus  string
		wantOpen    string
		wantQuit    string
	}{
		{
			name:        "en idle",
			lang:        "en",
			wantActive:  false,
			wantRunning: false,
			wantStatus:  "Status: Idle",
			wantOpen:    "Open AnyProfile",
			wantQuit:    "Quit AnyProfile",
		},
		{
			name:        "en single browser with api",
			lang:        "en",
			browsers:    1,
			apiRunning:  true,
			wantActive:  true,
			wantRunning: true,
			wantStatus:  "Active: 1 browser · API on",
			wantOpen:    "Open AnyProfile",
			wantQuit:    "Quit AnyProfile",
		},
		{
			name:        "en browsers and mobile",
			lang:        "en",
			browsers:    2,
			mobile:      1,
			wantActive:  true,
			wantRunning: true,
			wantStatus:  "Active: 2 browsers, 1 mobile",
			wantOpen:    "Open AnyProfile",
			wantQuit:    "Quit AnyProfile",
		},
		{
			name:        "vi idle with api",
			lang:        "vi",
			apiRunning:  true,
			wantActive:  true,
			wantRunning: false,
			wantStatus:  "Trạng thái: Đang chờ · API bật",
			wantOpen:    "Mở AnyProfile",
			wantQuit:    "Thoát AnyProfile",
		},
		{
			name:        "vi active sessions",
			lang:        "vi",
			browsers:    2,
			mobile:      1,
			wantActive:  true,
			wantRunning: true,
			wantStatus:  "Đang mở: 2 trình duyệt, 1 mobile",
			wantOpen:    "Mở AnyProfile",
			wantQuit:    "Thoát AnyProfile",
		},
		{
			name:        "unknown language falls back to en and clamps negatives",
			lang:        "fr",
			browsers:    -1,
			mobile:      -2,
			wantActive:  false,
			wantRunning: false,
			wantStatus:  "Status: Idle",
			wantOpen:    "Open AnyProfile",
			wantQuit:    "Quit AnyProfile",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatTrayState(tc.lang, tc.browsers, tc.mobile, tc.apiRunning)
			if got.Active != tc.wantActive || got.HasRunning != tc.wantRunning ||
				got.StatusLabel != tc.wantStatus || got.OpenLabel != tc.wantOpen ||
				got.QuitLabel != tc.wantQuit || got.SettingsLabel == "" ||
				got.UpdatesLabel == "" || got.StopAllLabel == "" || got.Tooltip == "" {
				t.Fatalf("formatTrayState(%q, %d, %d, %v) = %+v", tc.lang, tc.browsers, tc.mobile, tc.apiRunning, got)
			}
		})
	}
}

func TestBeforeCloseIdleAndShutdown(t *testing.T) {
	tmp := t.TempDir()
	dirs := paths.Dirs{Root: tmp}
	if err := dirs.Ensure(); err != nil {
		t.Fatalf("dirs.Ensure: %v", err)
	}
	st, err := store.Open(filepath.Join(tmp, "test.db"), filepath.Join(tmp, "backups"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	l := launcher.New(st, nil, dirs, nil, nil, nil)
	app := NewApp()
	app.svc = &service.Service{
		Dirs:     dirs,
		Store:    st,
		Launcher: l,
	}

	if b, m := app.runningCounts(); b != 0 || m != 0 {
		t.Fatalf("expected 0 running sessions, got browsers=%d mobile=%d", b, m)
	}
	if prevent := app.beforeClose(context.Background()); prevent {
		t.Fatal("expected beforeClose to allow quit when no sessions are running")
	}

	app.shutdown(context.Background())
	app.shutdown(context.Background())
}

