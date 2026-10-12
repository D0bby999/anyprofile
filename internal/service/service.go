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

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/netproxy"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

// LocaleFunc returns Chrome's UI locale and language list for an exit country.
type LocaleFunc func(ctx context.Context, country string) (uiLocale string, languages []string, err error)

type Service struct {
	Dirs     paths.Dirs
	Store    *store.Store
	Secrets  secrets.Store
	Engine   engine.Engine
	Locale   LocaleFunc
	Launcher *launcher.Launcher
	Log      *slog.Logger
	// CheckProxy measures a proxy's exit; netproxy.Check outside tests.
	CheckProxy func(ctx context.Context, up netproxy.Upstream) (netproxy.Exit, error)
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

// FileVaultOn reports whether the system volume is encrypted. Profile data
// other than cookies (history, local storage) relies on it.
func FileVaultOn() bool {
	out, err := execOutput("/usr/bin/fdesetup", "status")
	return err == nil && strings.Contains(out, "FileVault is On")
}
