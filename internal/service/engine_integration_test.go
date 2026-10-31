// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/engine/anybrowser"
	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

// TestCreateProfileComposesWithTheRealEngine runs the whole app path: profile
// creation asks the installed engine what each seed composes, pins a machine
// this host can serve, and the stored seed still composes in full afterwards.
// realEngineService builds the service over the real installed candidate, with
// the release hashes computed from the app itself so any extracted build works.
func realEngineService(t *testing.T) (*Service, string, string) {
	app := os.Getenv("ANYPROFILE_TEST_ANYBROWSER_APP")
	root := t.TempDir()
	dirs := paths.Dirs{Root: root}
	if err := dirs.Ensure(); err != nil {
		t.Fatal(err)
	}
	rel := anybrowser.Builtin()[0]
	files := map[string]string{}
	prefix := rel.App + "/"
	for name := range rel.Files {
		sum, err := sha256File(filepath.Join(app, strings.TrimPrefix(name, prefix)))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = sum
	}
	rel.Files = files
	eng := anybrowser.New(dirs)
	eng.SetCatalog(engine.NewCatalog("", rel))
	eng.MockKeychain = true
	if err := os.MkdirAll(filepath.Dir(eng.AppPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(app, eng.AppPath()); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dirs.DB(), dirs.Backups())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := &Service{
		Dirs: dirs, Store: st, Secrets: secrets.NewMemory(), Log: log,
		Engines:   map[string]engine.Engine{"anybrowser": eng},
		Preferred: "anybrowser",
		Launcher:  launcher.New(st, secrets.NewMemory(), dirs, nil, nil, log),
		Locale: func(context.Context, string) (string, []string, error) {
			return "en-US", []string{"en-US", "en"}, nil
		},
	}
	return s, rel.Version, filepath.Join(dirs.Engine("anybrowser", rel.Version), filepath.FromSlash(rel.Bin))
}

func TestCreateProfileComposesWithTheRealEngine(t *testing.T) {
	s, version, bin := realEngineService(t)
	p, err := s.CreateProfile(context.Background(), NewProfile{Name: "Composed"})
	if err != nil {
		t.Fatal(err)
	}
	if p.SeedHex == "" || len(p.SeedHex) != 16 {
		t.Fatalf("seed = %q, want the engine's own 16-hex seed", p.SeedHex)
	}
	if p.EngineMeta.HWC <= 0 || p.EngineMeta.MemGiB <= 0 {
		t.Fatalf("engine meta = %+v, want a composed machine", p.EngineMeta)
	}
	if p.EngineVersion != version {
		t.Fatalf("version = %q, want %q", p.EngineVersion, version)
	}
	// The seed stored with the profile must still compose in full on its own.
	dir, err := os.MkdirTemp("", "compose-explain-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin,
		"--fingerprint="+p.SeedHex, "--fingerprint-platform="+anybrowser.HostPlatform,
		"--fingerprint-explain", "--disable-gpu", "--no-first-run",
		"--hide-crash-restore-bubble", "--user-data-dir="+dir)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("explain the stored seed: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "cores-") || strings.Contains(text, "no option is servable") {
		t.Fatalf("stored seed does not compose fully:\n%s", text)
	}
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// TestCreateProfilesBulkComposesWithTheRealEngine checks the bulk path: every
// profile asks the engine for its own servable seed, the seeds differ, and the
// run stays quick enough for the UI.
func TestCreateProfilesBulkComposesWithTheRealEngine(t *testing.T) {
	s, _, _ := realEngineService(t)
	started := time.Now()
	out, err := s.CreateProfiles(context.Background(), BulkCreate{
		BaseName: "Bulk", Count: 5, Proxy: "none", Sites: []string{"other"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 5 {
		t.Fatalf("created %d profiles, want 5", len(out))
	}
	seen := map[string]bool{}
	for _, p := range out {
		if len(p.SeedHex) != 16 || p.EngineMeta.HWC <= 0 || p.EngineMeta.MemGiB <= 0 {
			t.Fatalf("profile %s: seed=%q meta=%+v", p.Name, p.SeedHex, p.EngineMeta)
		}
		if seen[p.SeedHex] {
			t.Fatalf("profile %s reuses seed %s", p.Name, p.SeedHex)
		}
		seen[p.SeedHex] = true
	}
	elapsed := time.Since(started)
	if elapsed > 60*time.Second {
		t.Fatalf("5 profiles took %s", elapsed)
	}
	t.Logf("5 profiles in %s", elapsed.Round(time.Millisecond))
}
