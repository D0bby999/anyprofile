// SPDX-License-Identifier: AGPL-3.0-or-later

package fury

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/store"
)

// configCLI runs the bundled fury-config helper by absolute path (never via
// PATH). Its inputs are not secret; its output stays in memory.
func (f *Fury) configCLI(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.configBin, args...)
	cmd.Env = []string{}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("fury-config %s: %v: %s", args[0], err, bytes.TrimSpace(errb.Bytes()))
	}
	return out.Bytes(), nil
}

// NewIdentity draws a macOS persona by population weight. Only macOS personas
// are offered on a Mac host: a Windows persona leaks the host's speech voices.
func (f *Fury) NewIdentity(ctx context.Context, roll uint64) (engine.Identity, error) {
	b, err := f.configCLI(ctx, "pick", "--os", "macos", "--roll", strconv.FormatUint(roll, 10))
	if err != nil {
		return engine.Identity{}, err
	}
	var r struct {
		PersonaID string `json:"personaId"`
	}
	if err := json.Unmarshal(b, &r); err != nil || r.PersonaID == "" {
		return engine.Identity{}, fmt.Errorf("fury-config pick: bad output")
	}
	return engine.Identity{PersonaID: r.PersonaID, Platform: "macos"}, nil
}

// UsesCookieKey is true: Fury reads a per-profile cookie key from a pipe.
func (f *Fury) UsesCookieKey() bool { return true }

// Locale is Chrome's own UI locale and language list for an exit country.
type Locale struct {
	UILocale  string   `json:"uiLocale"`
	Languages []string `json:"languages"`
}

func (f *Fury) LocaleFor(ctx context.Context, country string) (Locale, error) {
	var l Locale
	b, err := f.configCLI(ctx, "locale", "--country", country)
	if err != nil {
		return l, err
	}
	return l, json.Unmarshal(b, &l)
}

// Persona is one catalogue entry, for display.
type Persona struct {
	ID     string  `json:"id"`
	Screen string  `json:"screen"`
	GPU    string  `json:"gpu"`
	Weight float64 `json:"weight"`
}

func (f *Fury) Personas(ctx context.Context) ([]Persona, error) {
	var out []Persona
	b, err := f.configCLI(ctx, "personas", "--os", "macos")
	if err != nil {
		return nil, err
	}
	return out, json.Unmarshal(b, &out)
}

// deriveConfig produces the core's fingerprint config for a profile. The
// same profile always yields the same bytes.
// chromium is the core's version, for the UA and client hints.
func (f *Fury) deriveConfig(ctx context.Context, p store.Profile, chromium string) ([]byte, error) {
	major, _, _ := strings.Cut(chromium, ".")
	args := []string{"derive", "--persona", p.PersonaID, "--seed", p.SeedHex,
		"--timezone", p.Timezone, "--languages", p.Languages, "--ui-locale", p.UILocale,
		"--chrome-major", major, "--chrome-full", chromium}
	if p.Lat != nil && p.Lon != nil {
		args = append(args, "--lat", strconv.FormatFloat(*p.Lat, 'f', -1, 64),
			"--lon", strconv.FormatFloat(*p.Lon, 'f', -1, 64))
	}
	return f.configCLI(ctx, args...)
}
