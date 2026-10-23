// SPDX-License-Identifier: AGPL-3.0-or-later

// Package service is AnyProfile's application logic: what the UI can ask for,
// expressed over store, secrets, engine, netproxy and launcher. The Wails
// bindings in package main are a thin layer over it.
package service

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/control"
	"github.com/D0bby999/anyprofile/internal/netproxy"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/pro"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

// LocaleFunc returns Chrome's UI locale and language list for an exit country.
type LocaleFunc func(ctx context.Context, country string) (uiLocale string, languages []string, err error)

type Service struct {
	Dirs    paths.Dirs
	Store   *store.Store
	Secrets secrets.Store
	// Engines are the installed-or-installable engines, by ID. Preferred is
	// the engine new profiles get when the user has not chosen one.
	Engines   map[string]engine.Engine
	Preferred string
	// Catalog holds the engines' releases; the signed list from latest.json
	// is added to it. Nil in tests that do not care.
	Catalog  *engine.Catalog
	Locale   LocaleFunc
	Launcher *launcher.Launcher
	// Mobile runs this OS's mobile devices; nil where there is no driver.
	Mobile *mobile.Manager
	Log    *slog.Logger
	// Clock is nil in production (time.Now); tests fix it.
	Clock func() time.Time
	// OnChange is told the kind of data a write changed: "profiles",
	// "proxies", "sites", "settings" or "mobile". Nil in tests that do not care.
	OnChange func(kind string)
	// CheckProxy measures a proxy's exit; netproxy.Check outside tests.
	CheckProxy func(ctx context.Context, up netproxy.Upstream) (netproxy.Exit, error)
	// Pro reaches the AnyProfile Pro server; the zero value means pro.NewClient().
	Pro pro.Client

	snapMu    sync.Mutex
	updMu     sync.Mutex // one update download at a time
	proMu     sync.Mutex // one Pro check or install at a time
	ctlMu     sync.Mutex
	controls  map[string]*control.Device // the agent's handle on each running mobile device
	snapLocks map[string]profileLock     // one per profile: snapshots and launches take turns
}

func (s *Service) notify(kind string) {
	if s.OnChange != nil {
		s.OnChange(kind)
	}
}

func (s *Service) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now()
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand does not fail on macOS
	}
	return hex.EncodeToString(b)
}

func randomU64() uint64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return binary.BigEndian.Uint64(b[:])
}

// localZone is the Mac's own IANA zone, canonicalized, for profiles without a proxy.
func localZone() string {
	if target, err := os.Readlink("/etc/localtime"); err == nil {
		if _, zone, ok := strings.Cut(target, "zoneinfo/"); ok {
			if z := netproxy.NormalizeTimezone("", zone); z != "" {
				return z
			}
		}
	}
	return "UTC"
}
