// SPDX-License-Identifier: AGPL-3.0-or-later

package apostate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/engine/anybrowser"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/store"
)

func TestIndependentEngine(t *testing.T) {
	dirs := paths.Dirs{Root: t.TempDir()}
	a, fork := New(dirs), anybrowser.New(dirs)
	cat := engine.NewCatalog("", append(anybrowser.Builtin(), Builtin()...)...)
	a.SetCatalog(cat)
	fork.SetCatalog(cat)
	for _, e := range []engine.Engine{a, fork} {
		rels := e.Releases()
		if len(rels) != 1 || rels[0].Engine != e.ID() {
			t.Fatalf("%s releases = %+v", e.ID(), rels)
		}
	}
	ctx := context.Background()
	id, err := a.NewIdentity(ctx, 1)
	if err != nil || id.PersonaID != HostPersona || id.Platform != HostPlatform || id.Meta.HWC <= 0 || id.Meta.MemGiB <= 0 {
		t.Fatalf("identity = %+v, error = %v", id, err)
	}
	p := store.Profile{ID: "p", Engine: "apostate", EngineVersion: Version, PersonaID: id.PersonaID,
		Platform: id.Platform, SeedHex: "00000000000a11ce", Timezone: "Asia/Taipei", Languages: "zh-TW,zh,en", EngineMeta: id.Meta}
	spec, err := a.BuildLaunch(ctx, p, engine.Runtime{UserDataDir: "/profiles/p", RelayPort: 41000, CDPPipe: true})
	if err != nil {
		t.Fatal(err)
	}
	r := Builtin()[0]
	if spec.Binary != filepath.Join(dirs.Engine("apostate", Version), filepath.FromSlash(r.Bin)) || a.AppPath() == fork.AppPath() {
		t.Fatalf("engine paths are not independent: %+v", spec)
	}
	for _, arg := range []string{"--fingerprint=" + p.SeedHex, "--fingerprint-platform=" + HostPlatform,
		"--fingerprint-timezone=" + p.Timezone, "--fingerprint-locale=" + p.Languages,
		"--proxy-server=http://127.0.0.1:41000", "--fingerprint-webrtc-udp=block", "--remote-debugging-pipe"} {
		if !slices.Contains(spec.Args, arg) {
			t.Errorf("missing launch flag %s", arg)
		}
	}
	for _, key := range []string{"GOOGLE_API_KEY", "GOOGLE_DEFAULT_CLIENT_ID", "GOOGLE_DEFAULT_CLIENT_SECRET"} {
		if !slices.Contains(spec.Env, key+"=no") || !slices.Contains(spec.EnvUnset, key) {
			t.Errorf("legacy key policy missing for %s", key)
		}
	}
	if st := a.Status(ctx, "unknown"); st.Engine != "apostate" || st.Problem == "" {
		t.Fatalf("unknown status = %+v", st)
	}
	p.EngineVersion = "unknown"
	if _, err := a.BuildLaunch(ctx, p, engine.Runtime{}); !errors.Is(err, engine.ErrUnknownRelease) {
		t.Fatalf("unknown release error = %v", err)
	}
}

func TestRemoveDoesNotTouchAnyBrowser(t *testing.T) {
	dirs := paths.Dirs{Root: t.TempDir()}
	for _, id := range []string{"apostate", "anybrowser"} {
		dir := dirs.Engine(id, Version)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "keep"), []byte(id), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := New(dirs).Remove(Version); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dirs.Engine("apostate", Version)); !os.IsNotExist(err) {
		t.Fatalf("Apostate directory still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dirs.Engine("anybrowser", Version), "keep")); err != nil {
		t.Fatalf("AnyBrowser install changed: %v", err)
	}
}

func TestFutureReleaseDoesNotInheritGoogleWorkaround(t *testing.T) {
	a := New(paths.Dirs{Root: t.TempDir()})
	next := Builtin()[0]
	next.Version = "999.1"
	a.SetCatalog(engine.NewCatalog("", Builtin()[0], next))
	spec, err := a.BuildLaunch(context.Background(), store.Profile{ID: "p", EngineVersion: next.Version,
		SeedHex: "00000000000a11ce"}, engine.Runtime{})
	if err != nil {
		t.Fatal(err)
	}
	for _, env := range spec.Env {
		if strings.HasPrefix(env, "GOOGLE_") {
			t.Fatalf("future release inherited legacy workaround: %s", env)
		}
	}
	if len(spec.EnvUnset) != 3 {
		t.Fatal("inherited Google credentials are not stripped")
	}
}

func TestBuiltinUsesOfficialRelease(t *testing.T) {
	r := Builtin()[0]
	if r.Engine != "apostate" || r.Version != "0.5.1" || r.Chromium != "155.0.8059.31" ||
		r.URL != "https://github.com/heretic-tech/apostate/releases/download/v0.5.1/"+zipName ||
		r.Catalogue != Catalogue || len(r.Files) != 2 {
		t.Fatalf("not the pinned upstream release: %+v", r)
	}
	wantSHA := "58176ba9b3e9cfd74029165bdf716c636e618af17a48bd905fe6d5f1552d901a"
	if HostPlatform == "windows" {
		wantSHA = "8e55e44fb7f58619461e1d42a947446dcc08a2b46493c420c8e62ae27ab9c7e5"
	} else if r.TeamID != "PX3P7V373K" {
		t.Fatalf("macOS signing team = %q", r.TeamID)
	}
	if r.SHA256 != wantSHA {
		t.Fatalf("archive SHA = %q, want %q", r.SHA256, wantSHA)
	}
}
