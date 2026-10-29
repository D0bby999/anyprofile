// SPDX-License-Identifier: AGPL-3.0-or-later

// launch-built opens one throwaway profile on an AnyBrowser build that is not
// in any published list yet, through AnyProfile's real launcher: the archive
// from pro/chrome/scripts/package.sh and its .release.json. It prints the
// DevTools port; closing stdin stops it. Developer tool, not part of the app.
// -installed runs a compiled-in release from an existing install instead, for
// a side-by-side baseline under the same launcher.
//
//	go run ./tools/bench/launch-built -zip <archive.zip> -seed <hex16> < /tmp/ctl
//	go run ./tools/bench/launch-built -installed <engines/anybrowser/version> -seed <hex16> < /tmp/ctl
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/engine/anybrowser"
	"github.com/D0bby999/anyprofile/internal/engine/fury"
	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/netproxy"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/service"
	"github.com/D0bby999/anyprofile/internal/store"
)

func main() {
	zip := flag.String("zip", "", "archive from package.sh; <zip>.release.json must sit beside it")
	installed := flag.String("installed", "", "an installed compiled-in release's folder, instead of -zip")
	seed := flag.String("seed", "00000000000003e8", "16 hex digits")
	tz := flag.String("tz", "Asia/Ho_Chi_Minh", "timezone when there is no proxy")
	proxyURL := flag.String("proxy", os.Getenv("ANYPROFILE_PROXY"), "upstream proxy URL")
	flag.Parse()
	if err := run(*zip, *installed, *seed, *tz, *proxyURL); err != nil {
		fmt.Fprintln(os.Stderr, "launch-built:", err)
		os.Exit(1)
	}
}

func run(zip, installed, seed, tz, proxyURL string) error {
	var rel engine.Release
	if installed != "" {
		rel = anybrowser.Builtin()[0]
	} else {
		raw, err := os.ReadFile(zip + ".release.json")
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &rel); err != nil {
			return err
		}
	}
	if rel.Engine != "anybrowser" {
		return fmt.Errorf("unsupported engine %q: this helper requires an AnyBrowser release", rel.Engine)
	}
	root, err := os.MkdirTemp("/tmp", "apbuilt")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	dirs := paths.Dirs{Root: root}
	if err := dirs.Ensure(); err != nil {
		return err
	}
	versionDir := dirs.Engine("anybrowser", rel.Version)
	cat := engine.NewCatalog("9999.0.0", anybrowser.Builtin()...)
	if installed != "" {
		os.MkdirAll(filepath.Dir(versionDir), 0o700)
		if err := os.Symlink(installed, versionDir); err != nil {
			return err
		}
	} else {
		if err := engine.ExtractZip(zip, versionDir); err != nil {
			return err
		}
		cat = engine.NewCatalog("9999.0.0", append(anybrowser.Builtin(), rel)...)
	}
	ap := anybrowser.New(dirs)
	ap.SetCatalog(cat)
	ctx := context.Background()
	if st := ap.Status(ctx, rel.Version); !st.Installed {
		return fmt.Errorf("build not installed: %s", st.Problem)
	}

	st, err := store.Open(dirs.DB(), dirs.Backups())
	if err != nil {
		return err
	}
	defer st.Close()
	sec := secrets.NewMemory()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	engines := map[string]engine.Engine{"anybrowser": ap}
	l := launcher.New(st, sec, dirs, []engine.Engine{ap}, nil, log)
	// The app takes each country's locale from fury-config, for every engine.
	cfgBin, _ := filepath.Abs("tools/fury-config/target/release/fury-config")
	f := fury.New(dirs, cfgBin)
	svc := &service.Service{Dirs: dirs, Store: st, Secrets: sec, Launcher: l, Log: log,
		Engines: engines, Preferred: "anybrowser", Catalog: cat, CheckProxy: netproxy.Check,
		Locale: func(ctx context.Context, cc string) (string, []string, error) {
			loc, err := f.LocaleFor(ctx, cc)
			return loc.UILocale, loc.Languages, err
		}}
	proxyID := ""
	if proxyURL != "" {
		res, err := svc.ImportProxies(ctx, proxyURL, "")
		if err != nil || res.Added != 1 {
			return fmt.Errorf("proxy import failed: %v %+v", err, res.Failed)
		}
		rows, _ := svc.ListProxies()
		proxyID = rows[0].ID
	}
	p, err := svc.CreateProfile(ctx, service.NewProfile{Name: "bench", ProxyID: proxyID})
	if err != nil {
		return err
	}
	// A chosen seed on the build under test: copy the derived row.
	q := p
	q.ID, q.SeedHex, q.EngineVersion, q.CDPNextLaunch = p.ID+"b", seed, rel.Version, true
	if proxyID == "" {
		q.Timezone = tz
	}
	if err := st.CreateProfile(&q); err != nil {
		return err
	}
	if err := l.Start(ctx, q.ID); err != nil {
		return err
	}
	in := l.List()[0]
	got, _ := st.GetProfile(q.ID)
	fmt.Printf("port=%d pid=%d engine=%s@%s persona=%s tz=%s lang=%s\n", in.CDPPort, in.PID,
		got.Engine, got.EngineVersion, got.PersonaID, got.Timezone, got.Languages)
	io.Copy(io.Discard, os.Stdin)
	return l.Stop(q.ID)
}
