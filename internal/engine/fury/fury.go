// SPDX-License-Identifier: AGPL-3.0-or-later

package fury

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/store"
)

// Fury implements engine.Engine.
type Fury struct {
	dirs      paths.Dirs
	configBin string // absolute path to the bundled fury-config
	mu        sync.Mutex

	// Overridable in tests only.
	tarballURL, tarballSHA, binarySHA string
	skipCodesign                      bool
	chromeApps                        []string // where to look for Chrome's Widevine CDM
}

// New returns the engine. configBin must be an absolute path.
func New(dirs paths.Dirs, configBin string) *Fury {
	return &Fury{dirs: dirs, configBin: configBin, tarballURL: tarballURL,
		tarballSHA: tarballSHA256, binarySHA: binarySHA256, chromeApps: defaultChromeApps()}
}

func (f *Fury) ID() string { return "fury" }

// PrepareProfile stages the Widevine CDM from the user's own Chrome, when
// present. Missing Chrome is not fatal: DRM video simply will not play.
func (f *Fury) PrepareProfile(p store.Profile, rt engine.Runtime) error {
	err := stageWidevine(f.chromeApps, rt.UserDataDir)
	if errors.Is(err, errNoChrome) {
		return nil
	}
	return err
}

// BuildLaunch mirrors the switches the Fury agent passes its core
// (agent/src/launcher.rs), minus everything AnyProfile does not use.
// windowName prefixes the table number so windows of many profiles are told apart.
func windowName(p store.Profile) string {
	if p.Serial > 0 {
		return fmt.Sprintf("#%d %s", p.Serial, p.Name)
	}
	return p.Name
}

func (f *Fury) BuildLaunch(ctx context.Context, p store.Profile, rt engine.Runtime) (engine.LaunchSpec, error) {
	if len(rt.CookieKey) != 32 {
		return engine.LaunchSpec{}, fmt.Errorf("cookie key must be 32 bytes")
	}
	cfg, err := f.deriveConfig(ctx, p)
	if err != nil {
		return engine.LaunchSpec{}, err
	}
	args := []string{
		"--user-data-dir=" + rt.UserDataDir,
		"--no-default-browser-check",
		"--no-first-run",
		"--restore-last-session",
		"--disable-background-networking",
		"--disable-domain-reliability",
		"--disable-breakpad",
		"--safebrowsing-disable-auto-update",
		"--disable-field-trial-config",
		"--disk-cache-size=20971520",
		"--lang=" + p.UILocale,
		"--window-name=" + windowName(p),
		"--fury-fp-fd=3",
		"--fury-oscrypt-fd=4",
	}
	if rt.RelayPort > 0 {
		args = append(args,
			"--proxy-server=http://127.0.0.1:"+strconv.Itoa(rt.RelayPort),
			"--proxy-bypass-list=<-loopback>",
			"--disable-quic", // QUIC cannot ride an HTTP proxy and would go direct
			"--force-webrtc-ip-handling-policy=disable_non_proxied_udp",
		)
	}
	if rt.CDP {
		// Loopback only, and deliberately no --remote-allow-origins.
		args = append(args, "--remote-debugging-port=0")
	}
	args = append(args, rt.StartURLs...)
	return engine.LaunchSpec{
		Binary: f.BinaryPath(),
		Args:   args,
		Env:    []string{"TZ=" + p.Timezone},
		Pipes:  [][]byte{cfg, rt.CookieKey},
	}, nil
}

var _ engine.Engine = (*Fury)(nil)
