// SPDX-License-Identifier: AGPL-3.0-or-later

// launch-one opens one throwaway profile through AnyProfile's real launcher,
// with the diagnostics port on, and prints the port. Closing stdin stops it.
// Developer tool for measuring what users actually get; not part of the app.
//
//	go run ./tools/bench/launch-one -app <Fury.app> -persona <id> -seed <hex16> \
//	   [-tz Asia/Ho_Chi_Minh] [-proxy http://user:pass@host:port]   (or env ANYPROFILE_PROXY)
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/engine/fury"
	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/netproxy"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/service"
	"github.com/D0bby999/anyprofile/internal/store"
)

func main() {
	app := flag.String("app", "", "extracted Fury.app")
	persona := flag.String("persona", "macos-m4-1710x1112", "persona id")
	seed := flag.String("seed", "00000000000003e8", "16 hex digits")
	tz := flag.String("tz", "Asia/Ho_Chi_Minh", "timezone when there is no proxy")
	proxyURL := flag.String("proxy", os.Getenv("ANYPROFILE_PROXY"), "upstream proxy URL")
	flag.Parse()
	if err := run(*app, *persona, *seed, *tz, *proxyURL); err != nil {
		fmt.Fprintln(os.Stderr, "launch-one:", err)
		os.Exit(1)
	}
}

func run(app, persona, seed, tz, proxyURL string) error {
	root, err := os.MkdirTemp("/tmp", "ap1")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	dirs := paths.Dirs{Root: root}
	if err := dirs.Ensure(); err != nil {
		return err
	}
	os.MkdirAll(dirs.Engine("fury", fury.Version), 0o700)
	if err := os.Symlink(app, filepath.Join(dirs.Engine("fury", fury.Version), "Fury.app")); err != nil {
		return err
	}
	st, err := store.Open(dirs.DB(), dirs.Backups())
	if err != nil {
		return err
	}
	defer st.Close()
	cfgBin, _ := filepath.Abs("tools/fury-config/target/release/fury-config")
	f := fury.New(dirs, cfgBin)
	sec := secrets.NewMemory()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	l := launcher.New(st, sec, dirs, []engine.Engine{f}, nil, log)
	svc := &service.Service{Dirs: dirs, Store: st, Secrets: sec, Launcher: l, Log: log,
		Engines: map[string]engine.Engine{"fury": f}, Preferred: "fury",
		CheckProxy: netproxy.Check,
		Locale: func(ctx context.Context, cc string) (string, []string, error) {
			loc, err := f.LocaleFor(ctx, cc)
			return loc.UILocale, loc.Languages, err
		}}
	ctx := context.Background()
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
	// The bench needs a chosen identity, which the app never lets anyone edit:
	// copy the network fields the service derived into a new row.
	q := p
	q.ID, q.PersonaID, q.SeedHex, q.CDPNextLaunch = p.ID+"b", persona, seed, true
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
	fmt.Printf("port=%d pid=%d persona=%s tz=%s lang=%s\n", in.CDPPort, in.PID, got.PersonaID, got.Timezone, got.Languages)
	io.Copy(io.Discard, os.Stdin)
	return l.Stop(q.ID)
}
