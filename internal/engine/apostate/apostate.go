// SPDX-License-Identifier: AGPL-3.0-or-later

package apostate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/store"
)

// Apostate implements engine.Engine.
type Apostate struct {
	dirs paths.Dirs
	mu   sync.Mutex
	// MockKeychain adds --use-mock-keychain. Integration tests only: the app
	// never sets it, because the mock key is public (decision 3 in the plan).
	MockKeychain bool
	// host reports logical cores and installed GiB; nil means the real machine.
	host func() (cores, memGiB int)
}

func New(dirs paths.Dirs) *Apostate { return &Apostate{dirs: dirs} }

func (a *Apostate) ID() string { return "apostate" }

// UsesCookieKey is false: Apostate keeps its cookie key itself, in the
// Keychain item "Chromium Safe Storage" on macOS and DPAPI-encrypted in the
// profile's Local State on Windows, and never reads one from AnyProfile.
func (a *Apostate) UsesCookieKey() bool { return false }

// ProbeSeesPrimaryLanguageOnly is true: measured 2026-10-14, a hidden tab
// reports "vi" for --fingerprint-locale=vi-VN,vi,en-US,en while visible tabs
// report the whole list.
func (a *Apostate) ProbeSeesPrimaryLanguageOnly() bool { return true }

func (a *Apostate) hostInfo() (int, int) {
	if a.host != nil {
		return a.host()
	}
	return runtime.NumCPU(), int(hostMemBytes() >> 30)
}

// NewIdentity pins the core count and memory of this computer. The machine
// itself comes from the profile's seed (seed_hex), drawn by the service, so
// roll is unused. The persona is the host's own OS (release_<os>.go): another
// OS's persona renders with the host's fonts, which detection sites notice.
func (a *Apostate) NewIdentity(ctx context.Context, roll uint64) (engine.Identity, error) {
	cores, mem := a.hostInfo()
	return engine.Identity{PersonaID: personaID, Platform: platform,
		Meta: store.EngineMeta{HWC: cores, MemGiB: mem}}, nil
}

// PrepareProfile blocks geolocation before the first launch: Apostate has no
// flag to fake a position, so a site that asks would get the real one.
func (a *Apostate) PrepareProfile(p store.Profile, rt engine.Runtime) error {
	return blockGeolocation(filepath.Join(rt.UserDataDir, "Default", "Preferences"))
}

func blockGeolocation(prefs string) error {
	m := map[string]any{}
	b, err := os.ReadFile(prefs)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(filepath.Dir(prefs), 0o700); err != nil {
			return err
		}
	case err != nil:
		return err
	default:
		if err := json.Unmarshal(b, &m); err != nil {
			return fmt.Errorf("read %s: %w", prefs, err)
		}
	}
	profile, _ := m["profile"].(map[string]any)
	if profile == nil {
		profile = map[string]any{}
		m["profile"] = profile
	}
	values, _ := profile["default_content_setting_values"].(map[string]any)
	if values == nil {
		values = map[string]any{}
		profile["default_content_setting_values"] = values
	}
	if v, ok := values["geolocation"].(float64); ok && v == 2 {
		return nil
	}
	values["geolocation"] = 2
	out, err := json.Marshal(m)
	if err != nil {
		return err
	}
	tmp := prefs + ".anyprofile-tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, prefs)
}

var seedRe = regexp.MustCompile(`^[0-9a-f]{16}$`)

// ErrOtherOS refuses a profile created on another operating system.
var ErrOtherOS = errors.New("this profile opens only on the operating system it was created on")

func (a *Apostate) BuildLaunch(ctx context.Context, p store.Profile, rt engine.Runtime) (engine.LaunchSpec, error) {
	if !seedRe.MatchString(p.SeedHex) {
		return engine.LaunchSpec{}, fmt.Errorf("profile %s has an invalid seed", p.ID)
	}
	// A persona of another OS would render with this host's fonts and GPU.
	if p.Platform != "" && p.Platform != platform {
		return engine.LaunchSpec{}, fmt.Errorf("%w: it was created on %s", ErrOtherOS, p.Platform)
	}
	// Without these three, Chromium shows a "Google API keys are missing" bar
	// on the first tab; the page can measure the lost height.
	env := []string{"GOOGLE_API_KEY=no", "GOOGLE_DEFAULT_CLIENT_ID=no", "GOOGLE_DEFAULT_CLIENT_SECRET=no"}
	args := []string{
		"--user-data-dir=" + rt.UserDataDir,
		"--no-default-browser-check",
		"--no-first-run",
	}
	if !rt.FreshStart {
		args = append(args, "--restore-last-session")
	}
	args = append(args,
		// Quiet the background traffic a fresh Chromium makes on its own.
		"--disable-background-networking",
		"--disable-domain-reliability",
		"--disable-breakpad",
		"--safebrowsing-disable-auto-update",
		"--disable-field-trial-config",
		"--disk-cache-size=20971520",
		// A covered window keeps painting, so an agent or script working
		// behind the user's apps gets its clicks and screenshots through.
		"--disable-backgrounding-occluded-windows",
		"--window-name="+engine.WindowName(p),
		// The seed fixes the machine; the platform keeps it the host's OS.
		"--fingerprint="+p.SeedHex,
		"--fingerprint-platform="+platform,
	)
	if p.Timezone != "" {
		// Apostate's own default is the host zone even behind a proxy.
		args = append(args, "--fingerprint-timezone="+p.Timezone)
		if setsTZ {
			env = append([]string{"TZ=" + p.Timezone}, env...)
		}
	}
	if p.Languages != "" {
		// --lang has no effect on navigator.languages in Apostate; this does.
		args = append(args, "--fingerprint-locale="+p.Languages)
	}
	// Pinned at creation so the seed keeps the same values on another host or
	// after a persona-table change. Apostate refuses values above the host's.
	cores, mem := a.hostInfo()
	if p.EngineMeta.HWC > 0 {
		args = append(args, "--fingerprint-hardware-concurrency="+strconv.Itoa(min(p.EngineMeta.HWC, cores)))
	}
	if p.EngineMeta.MemGiB > 0 {
		args = append(args, "--fingerprint-device-memory="+strconv.Itoa(min(p.EngineMeta.MemGiB, mem)))
	}
	if rt.RelayPort > 0 {
		// The local relay holds the proxy credentials; Apostate's own proxy
		// auth would hand them to child processes in argv.
		args = append(args,
			"--proxy-server=http://127.0.0.1:"+strconv.Itoa(rt.RelayPort),
			"--proxy-bypass-list=<-loopback>",
			"--disable-quic",
			"--force-webrtc-ip-handling-policy=disable_non_proxied_udp",
			"--fingerprint-webrtc-udp=block",
		)
	}
	if a.MockKeychain {
		args = append(args, "--use-mock-keychain")
	}
	if rt.CDP {
		args = append(args, "--remote-debugging-port=0")
	}
	if rt.CDPPipe {
		args = append(args, "--remote-debugging-pipe")
	}
	if len(rt.Extensions) > 0 {
		args = append(args, "--load-extension="+strings.Join(rt.Extensions, ","))
	}
	args = append(args, rt.StartURLs...)
	return engine.LaunchSpec{Binary: a.BinaryPath(), Args: args, Env: env, CDPPipe: rt.CDPPipe}, nil
}
