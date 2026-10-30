// SPDX-License-Identifier: AGPL-3.0-or-later

// external-session opens an exact verified candidate through the real launcher.
// Test profiles only, mock Keychain, optional approved proxy from a private file;
// never opens the user's data or puts proxy credentials in process arguments.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

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

func validateDescriptor(rel engine.Release) error {
	if err := rel.Validate(); err != nil {
		return err
	}
	pin := anybrowser.Builtin()[0]
	if rel.Engine != pin.Engine || rel.Version != pin.Version || rel.Chromium != pin.Chromium ||
		rel.Platform != pin.Platform || rel.Catalogue != pin.Catalogue || rel.Bin != pin.Bin || rel.App != pin.App ||
		len(rel.Files) != len(pin.Files) {
		return fmt.Errorf("candidate descriptor identity/layout does not match this platform pin")
	}
	for file := range pin.Files {
		if len(rel.Files[file]) != 64 || rel.Files[file] == "0000000000000000000000000000000000000000000000000000000000000000" {
			return fmt.Errorf("candidate descriptor missing nonzero core hash: %s", file)
		}
	}
	return nil
}

func readProxyFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("proxy input file unavailable")
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() > 8192 {
		return "", fmt.Errorf("proxy input must be a private regular file (0600), at most 8192 bytes")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("proxy input could not be read")
	}
	text := strings.TrimSpace(string(data))
	lines := netproxy.ParseList(text)
	if len(lines) != 1 || lines[0].Err != "" {
		return "", fmt.Errorf("proxy input must contain exactly one valid proxy (value omitted)")
	}
	return text, nil
}

func sameProxyExit(expected store.Proxy, actual netproxy.Exit) bool {
	return expected.ExitIP != "" && expected.Country != "" && expected.Timezone != "" &&
		!actual.Rotating && actual.IP == expected.ExitIP && actual.IPv6 == expected.ExitIP6 &&
		actual.Country == expected.Country && actual.Timezone == expected.Timezone
}

func monitorExit(ctx context.Context, ticks <-chan time.Time, verify func(context.Context) error) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-ticks:
			if !ok {
				return fmt.Errorf("proxy monitor stopped; test window invalidated")
			}
			checkCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			err := verify(checkCtx)
			cancel()
			if ctx.Err() != nil {
				return nil
			}
			if err != nil {
				return err
			}
		}
	}
}

func run(ctx context.Context, app, descriptor, seed, startURL, proxyFile string, diagnostics bool) error {
	data, err := os.ReadFile(descriptor)
	if err != nil {
		return err
	}
	var rel engine.Release
	if err := json.Unmarshal(data, &rel); err != nil {
		return err
	}
	if err := validateDescriptor(rel); err != nil {
		return err
	}
	app, err = filepath.Abs(app)
	if err != nil {
		return err
	}
	root, err := os.MkdirTemp("/tmp", "apexternal")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	dirs := paths.Dirs{Root: root}
	if err := dirs.Ensure(); err != nil {
		return err
	}
	target := filepath.Join(dirs.Engine(rel.Engine, rel.Version), filepath.FromSlash(rel.App))
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	if err := os.Symlink(app, target); err != nil {
		return err
	}
	eng := anybrowser.New(dirs)
	catalog := engine.NewCatalog("", rel)
	eng.SetCatalog(catalog)
	if status := eng.Status(ctx, rel.Version); !status.Installed {
		return fmt.Errorf("candidate integrity check failed: %s", status.Problem)
	}
	eng.MockKeychain = true
	st, err := store.Open(dirs.DB(), dirs.Backups())
	if err != nil {
		return err
	}
	defer st.Close()
	// Never send proxy/auth-bearing errors to a general log. Fixture metadata
	// contains no URL credential; passwords live only in this process's Memory.
	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))
	sec := secrets.NewMemory()
	l := launcher.New(st, sec, dirs, []engine.Engine{eng}, nil, log)
	defer l.StopAll()
	p := store.Profile{ID: "external-audit", Name: "External audit", Serial: 1,
		Engine: rel.Engine, EngineVersion: rel.Version, PersonaID: anybrowser.HostPersona,
		Platform: anybrowser.HostPlatform, SeedHex: seed, CDPNextLaunch: diagnostics, StartURL: startURL,
		Timezone: "Asia/Ho_Chi_Minh", Languages: "en-US,en",
		EngineMeta: store.EngineMeta{HWC: 8, MemGiB: 16}}
	network, country, proxyPort := "direct-no-proxy", "", 0
	var verifyExit func(context.Context) error
	if proxyFile == "" {
		if err := st.CreateProfile(&p); err != nil {
			return err
		}
	} else {
		raw, err := readProxyFile(proxyFile)
		if err != nil {
			return err
		}
		config, err := filepath.Abs("tools/fury-config/target/release/fury-config")
		if err != nil {
			return err
		}
		locale := fury.New(dirs, config)
		svc := &service.Service{Dirs: dirs, Store: st, Secrets: sec, Launcher: l, Log: log,
			Engines: map[string]engine.Engine{eng.ID(): eng}, Preferred: eng.ID(), Catalog: catalog,
			CheckProxy: netproxy.Check, Locale: func(ctx context.Context, cc string) (string, []string, error) {
				v, err := locale.LocaleFor(ctx, cc)
				return v.UILocale, v.Languages, err
			}}
		l.BeforeStart, l.AfterExit = svc.BeforeLaunch, svc.AfterExit
		result, err := svc.ImportProxies(ctx, raw, "")
		if err != nil || result.Added != 1 || len(result.Unreachable) != 0 {
			return fmt.Errorf("proxy import/check did not succeed (credential-bearing details omitted)")
		}
		proxies, err := st.ListProxies()
		if err != nil || len(proxies) != 1 || proxies[0].Rotating {
			return fmt.Errorf("expected one verified sticky proxy")
		}
		px := proxies[0]
		if px.ExitIP == "" || px.Country == "" || px.Timezone == "" {
			return fmt.Errorf("proxy exit identity is incomplete; test window invalidated")
		}
		profileStartURL := startURL
		if profileStartURL == "about:blank" {
			profileStartURL = "" // Service intentionally accepts only http(s) start pages.
		}
		p, err = svc.CreateProfile(ctx, service.NewProfile{Name: "External proxy audit", Engine: eng.ID(), ProxyID: px.ID, StartURL: profileStartURL})
		if err != nil {
			return fmt.Errorf("proxy profile admission failed (details omitted)")
		}
		// Identity fields are immutable after insert. Make a separate owned test
		// fixture before insertion instead of changing the Service-created row.
		p.ID += "-fixture"
		p.Serial = 0
		p.SeedHex, p.EngineVersion, p.CDPNextLaunch = seed, rel.Version, diagnostics
		if err := st.CreateProfile(&p); err != nil {
			return err
		}
		network, country, proxyPort = "verified-sticky-proxy-via-app-relay", px.Country, px.Port
		upstream := netproxy.ParseList(raw)[0].Upstream
		verifyExit = func(checkCtx context.Context) error {
			exit, err := netproxy.Check(checkCtx, upstream)
			if err != nil {
				return fmt.Errorf("proxy health could not be revalidated; test window invalidated")
			}
			if !sameProxyExit(px, exit) {
				return fmt.Errorf("proxy exit identity changed; test window invalidated")
			}
			return nil
		}
	}
	if err := l.Start(ctx, p.ID); err != nil {
		if proxyFile != "" {
			return fmt.Errorf("proxy-backed launch failed (details omitted)")
		}
		return err
	}
	running := l.List()
	if len(running) != 1 || (diagnostics && running[0].CDPPort == 0) || (!diagnostics && running[0].CDPPort != 0) {
		return fmt.Errorf("candidate diagnostics state does not match the requested mode")
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]any{
		"port": running[0].CDPPort, "pid": running[0].PID, "version": rel.Version,
		"chromium": rel.Chromium, "seed": seed, "root": root, "keychain": "mock-test-only",
		"network": network, "timezone": p.Timezone, "languages": p.Languages,
		"proxyCountry": country, "proxyPort": proxyPort,
		"proxyMonitorEnabled": verifyExit != nil,
		"diagnostics":         diagnostics, "startURL": startURL,
	}); err != nil {
		return err
	}
	if verifyExit == nil {
		<-ctx.Done()
		return nil
	}
	// A sticky-port check at launch does not prove that a long test window is
	// stable. Abort rather than silently changing the running profile's identity.
	ticker := time.NewTicker(3 * time.Minute)
	defer ticker.Stop()
	return monitorExit(ctx, ticker.C, verifyExit)
}

func main() {
	app := flag.String("app", "", "exact verified Chromium.app")
	descriptor := flag.String("descriptor", "", "test-only release descriptor with core hashes")
	seed := flag.String("seed", "00000000000003e8", "16 hex digits")
	duration := flag.Duration("duration", 30*time.Minute, "maximum owned session lifetime")
	diagnostics := flag.Bool("diagnostics", true, "open a diagnostics port; false for native-only observations")
	startURL := flag.String("start-url", "about:blank", "initial URL in the owned test profile")
	proxyFile := flag.String("proxy-file", "", "private 0600 file containing one approved proxy; credentials never go in argv")
	flag.Parse()
	if *app == "" || *descriptor == "" || *duration <= 0 || *duration > 3*time.Hour {
		fmt.Fprintln(os.Stderr, "app and descriptor required; duration must be in (0, 3h]")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *duration)
	defer cancel()
	if err := run(ctx, *app, *descriptor, *seed, *startURL, *proxyFile, *diagnostics); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
