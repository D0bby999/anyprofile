// SPDX-License-Identifier: AGPL-3.0-or-later

// mkprofile creates one profile in the installed app's database, with the
// engines and proxy it already has. Developer tool: the app has no create
// endpoint, and a bench needs a profile that was composed by the current engine
// instead of one that predates it.
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
	"github.com/D0bby999/anyprofile/internal/engine/anybrowser"
	"github.com/D0bby999/anyprofile/internal/netproxy"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/service"
	"github.com/D0bby999/anyprofile/internal/store"
)

func main() {
	root := flag.String("root", "", "app data root (default: the installed app's)")
	proxyID := flag.String("proxy", "", "proxy id to use")
	name := flag.String("name", "Bench fresh", "profile name")
	flag.Parse()
	if *root == "" {
		home, _ := os.UserHomeDir()
		*root = filepath.Join(home, "Library", "Application Support", "AnyProfile")
	}
	if err := run(*root, *proxyID, *name); err != nil {
		fmt.Fprintln(os.Stderr, "mkprofile:", err)
		os.Exit(1)
	}
}

func run(root, proxyID, name string) error {
	dirs := paths.Dirs{Root: root}
	st, err := store.Open(dirs.DB(), dirs.Backups())
	if err != nil {
		return err
	}
	defer st.Close()
	sec := secrets.NewVault(dirs.Vault(), secrets.Keychain{})
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ab := anybrowser.New(dirs)
	svc := &service.Service{Dirs: dirs, Store: st, Secrets: sec, Log: log,
		Engines: map[string]engine.Engine{"anybrowser": ab}, Preferred: "anybrowser",
		CheckProxy: netproxy.Check,
		Locale: func(_ context.Context, cc string) (string, []string, error) {
			switch cc {
			case "VN":
				return "vi", []string{"vi-VN", "vi"}, nil
			case "DE":
				return "de", []string{"de-DE", "de"}, nil
			}
			return "en-US", []string{"en-US", "en"}, nil
		}}
	p, err := svc.CreateProfile(context.Background(), service.NewProfile{Name: name, ProxyID: proxyID})
	if err != nil {
		return err
	}
	fmt.Printf("%s\ttz=%s\tlat=%v\tlon=%v\n", p.ID, p.Timezone, p.Lat, p.Lon)
	return nil
}
