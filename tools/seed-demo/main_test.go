// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/service"
	"github.com/D0bby999/anyprofile/internal/store"
)

// TestSeedShowsEveryState seeds a temporary directory and reads it back the
// way the profile table does.
func TestSeedShowsEveryState(t *testing.T) {
	cfg, _ := filepath.Abs("../../build/bin/AnyProfile.app/Contents/Resources/fury-config")
	if _, err := os.Stat(cfg); err != nil {
		t.Skip("needs make build for the fury-config helper")
	}
	dir := filepath.Join(t.TempDir(), "demo")
	if err := run(dir, cfg, false); err != nil {
		t.Fatal(err)
	}
	if err := run(dir, cfg, false); err == nil {
		t.Fatal("a second run without -reset must refuse")
	}
	dirs := paths.Dirs{Root: dir}
	st, err := store.Open(dirs.DB(), dirs.Backups())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := &service.Service{Dirs: dirs, Store: st, Log: log,
		Launcher: launcher.New(st, secrets.Keychain{}, dirs, nil, func(launcher.Info) {}, log)}
	rows, err := svc.ListProfiles(store.ProfileFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 50 {
		t.Fatalf("only %d live profiles", len(rows))
	}
	seen := map[string]bool{}
	attention := 0 // mirrors needsAttention in frontend/src/lib/health.ts
	for _, r := range rows {
		bad := r.FPVerdict == "drift" || r.FPVerdict == "unchecked" || r.ProxyExpiry != ""
		for _, f := range r.Flags {
			bad = bad || f.Level == "warn" || f.Level == "danger"
		}
		for _, c := range r.Checks {
			bad = bad || !c.OK
		}
		if bad {
			attention++
		}
		for _, f := range r.Flags {
			seen["flag:"+f.ID] = true
		}
		for _, c := range r.Checks {
			if !c.OK {
				seen["check:"+c.ID] = true
			}
		}
		seen["verdict:"+r.FPVerdict] = true
		seen["expiry:"+r.ProxyExpiry] = true
		seen["engine:"+r.Engine] = true
		if r.Pinned {
			seen["pinned"] = true
		}
	}
	for _, want := range []string{"flag:direct", "flag:country_changed", "flag:ip_changed", "flag:tz_mismatch",
		"flag:exit_failed", "check:webrtc_ip", "check:langs", "verdict:drift", "verdict:match", "verdict:baseline",
		"verdict:unchecked", "verdict:", "expiry:soon", "expiry:expired", "engine:fury", "engine:apostate", "pinned"} {
		if !seen[want] {
			t.Errorf("no profile shows %s", want)
		}
	}
	// docs/manual-testing.md promises nine: every "Mẫu" profile except "đổi IP".
	if attention != 9 {
		t.Errorf("needs attention = %d, want 9", attention)
	}
	if err := run("", cfg, false); err == nil {
		t.Error("an empty -dir must refuse")
	}
	t.Setenv(paths.EnvDataDir, "")
	if real, err := paths.Default(); err == nil {
		if err := run(real.Root, cfg, true); err == nil || !strings.Contains(err.Error(), "real data") {
			t.Errorf("the real data directory must be refused: %v", err)
		}
	}
}
