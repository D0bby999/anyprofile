// SPDX-License-Identifier: AGPL-3.0-or-later

package anybrowser

import (
	"context"
	"errors"
	"fmt"
	"maps"
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

// AnyBrowser implements engine.Engine.
type AnyBrowser struct {
	dirs       paths.Dirs
	mu         sync.Mutex
	pinned     engine.Release
	persona    string
	legacyKeys bool
	// MockKeychain adds --use-mock-keychain. Integration tests only: the app
	// never sets it, because the mock key is public (decision 3 in the plan).
	MockKeychain bool
	// host reports logical cores and installed GiB; nil means the real machine.
	host func() (cores, memGiB int)
	cat  *engine.Catalog
	// composeSeeds is set on AnyBrowser only: upstream Apostate shares this
	// driver but must keep creating profiles exactly as it did.
	composeSeeds bool
}

// New returns the engine with the pinned release only; SetCatalog adds the
// signed list.
func New(dirs paths.Dirs) *AnyBrowser {
	a := NewWithRelease(dirs, Builtin()[0], personaID, legacyGoogleKeyEnv)
	a.composeSeeds = true
	return a
}

// NewWithRelease shares the Chromium launch/install implementation with the
// upstream Apostate engine. Its identity, release pins and install tree stay
// separate. legacyKeys enables the non-secret Google infobar workaround only
// for this exact pinned release, never for a future catalog entry.
func NewWithRelease(dirs paths.Dirs, pinned engine.Release, persona string, legacyKeys bool) *AnyBrowser {
	return &AnyBrowser{dirs: dirs, pinned: pinned, persona: persona, legacyKeys: legacyKeys,
		cat: engine.NewCatalog("", pinned)}
}

func (a *AnyBrowser) ID() string { return a.pinned.Engine }

// UsesCookieKey is false: AnyBrowser keeps its cookie key itself, in the
// Keychain item "Chromium Safe Storage" on macOS and DPAPI-encrypted in the
// profile's Local State on Windows, and never reads one from AnyProfile.
func (a *AnyBrowser) UsesCookieKey() bool { return false }

// ProbeSeesPrimaryLanguageOnly is true: measured 2026-10-14, a hidden tab
// reports "vi" for --fingerprint-locale=vi-VN,vi,en-US,en while visible tabs
// report the whole list.
func (a *AnyBrowser) ProbeSeesPrimaryLanguageOnly() bool { return true }

func (a *AnyBrowser) hostInfo() (int, int) {
	if a.host != nil {
		return a.host()
	}
	return runtime.NumCPU(), int(hostMemBytes() >> 30)
}

// NewIdentity pins the core count and memory of this computer. The machine
// itself comes from the profile's seed (seed_hex), drawn by the service, so
// roll is unused. The persona is the host's own OS (release_<os>.go): another
// OS's persona renders with the host's fonts, which detection sites notice.
func (a *AnyBrowser) NewIdentity(ctx context.Context, roll uint64) (engine.Identity, error) {
	return a.ComposeIdentity(ctx, a.pinned.Version, roll)
}

// PrepareProfile settles geolocation before the first launch. A profile with a
// position answers it -- the app sets one from the proxy exit -- and the
// permission is allowed, so a page that asks gets the profile's position
// instead of waiting for a prompt nobody is there to answer. A profile without
// one is denied, so a site never gets this computer's real position. It also
// keeps the browser out of Google sign-in (engine.StayOutOfChromeSignIn).
func (a *AnyBrowser) PrepareProfile(p store.Profile, rt engine.Runtime) error {
	_, _, _, hasFix := a.geolocationFix(p)
	return engine.EditPrefs(filepath.Join(rt.UserDataDir, "Default", "Preferences"), func(m map[string]any) bool {
		geo := configureGeolocation(m, hasFix)
		return engine.StayOutOfChromeSignIn(m) || geo
	})
}

func configureGeolocation(m map[string]any, allow bool) bool {
	values := engine.PrefsDict(engine.PrefsDict(m, "profile"), "default_content_setting_values")
	want := float64(2)
	if allow {
		want = 1
	}
	if v, ok := values["geolocation"].(float64); ok && v == want {
		return false
	}
	values["geolocation"] = want
	return true
}

// HasGeolocation reports whether AnyBrowser on this host can answer
// navigator.geolocation with p's coordinates instead of denying the permission.
func HasGeolocation(p store.Profile) bool {
	if !supportsGeolocation || p.Lat == nil || p.Lon == nil {
		return false
	}
	lat, lon := *p.Lat, *p.Lon
	return lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}

// geolocationFix returns the position the profile claims, when the engine and
// profile have one (it is copied from the proxy exit). The engine reports it
// instead of this computer's position, so the answer agrees with the exit IP
// and the timezone. Upstream Apostate (composeSeeds == false) lacks patch 0164
// and always denies geolocation.
func (a *AnyBrowser) geolocationFix(p store.Profile) (lat, lon, accuracy float64, ok bool) {
	if !a.composeSeeds || !HasGeolocation(p) {
		return 0, 0, 0, false
	}
	return *p.Lat, *p.Lon, 25, true
}

var seedRe = regexp.MustCompile(`^[0-9a-f]{16}$`)

// ErrOtherOS refuses a profile created on another operating system.
var ErrOtherOS = errors.New("this profile opens only on the operating system it was created on")

func (a *AnyBrowser) BuildLaunch(ctx context.Context, p store.Profile, rt engine.Runtime) (engine.LaunchSpec, error) {
	if !seedRe.MatchString(p.SeedHex) {
		return engine.LaunchSpec{}, fmt.Errorf("profile %s has an invalid seed", p.ID)
	}
	// A persona of another OS would render with this host's fonts and GPU.
	if p.Platform != "" && p.Platform != platform {
		return engine.LaunchSpec{}, fmt.Errorf("%w: it was created on %s", ErrOtherOS, p.Platform)
	}
	r, err := a.release(p.EngineVersion)
	if err != nil {
		return engine.LaunchSpec{}, err
	}
	// The macOS fork is keyless (patch 0159); upstream Apostate needs the
	// non-secret infobar workaround. Always strip inherited real keys and
	// never enable the workaround for a future/unrecognized release.
	var env []string
	pin := a.pinned
	if a.legacyKeys && r.Version == pin.Version && r.Chromium == pin.Chromium &&
		r.URL == pin.URL && r.SHA256 == pin.SHA256 && r.Catalogue == pin.Catalogue &&
		r.Bin == pin.Bin && r.App == pin.App && r.TeamID == pin.TeamID && maps.Equal(r.Files, pin.Files) {
		env = append(env, "GOOGLE_API_KEY=no", "GOOGLE_DEFAULT_CLIENT_ID=no", "GOOGLE_DEFAULT_CLIENT_SECRET=no")
	}
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
		// AnyBrowser's own default is the host zone even behind a proxy.
		args = append(args, "--fingerprint-timezone="+p.Timezone)
		if setsTZ {
			env = append([]string{"TZ=" + p.Timezone}, env...)
		}
	}
	if p.Languages != "" {
		// --lang has no effect on navigator.languages in AnyBrowser; this does.
		args = append(args, "--fingerprint-locale="+p.Languages)
	}
	// Pinned at creation so the seed keeps the same values on another host or
	// after a persona-table change. AnyBrowser refuses values above the host's.
	cores, mem := a.hostInfo()
	var warn []string
	if p.EngineMeta.HWC > 0 {
		if p.EngineMeta.HWC > cores {
			warn = append(warn, fmt.Sprintf("%d cores pinned, this machine has %d", p.EngineMeta.HWC, cores))
		}
		args = append(args, "--fingerprint-hardware-concurrency="+strconv.Itoa(min(p.EngineMeta.HWC, cores)))
	}
	if p.EngineMeta.MemGiB > 0 {
		if p.EngineMeta.MemGiB > mem {
			warn = append(warn, fmt.Sprintf("%d GiB pinned, this machine has %d", p.EngineMeta.MemGiB, mem))
		}
		args = append(args, "--fingerprint-device-memory="+strconv.Itoa(min(p.EngineMeta.MemGiB, mem)))
	}
	if rt.RelayPort > 0 {
		// The local relay holds the proxy credentials; AnyBrowser's own proxy
		// auth would hand them to child processes in argv.
		args = append(args,
			"--proxy-server=http://127.0.0.1:"+strconv.Itoa(rt.RelayPort),
			"--proxy-bypass-list=<-loopback>",
			"--disable-quic",
			"--force-webrtc-ip-handling-policy=disable_non_proxied_udp",
			"--fingerprint-webrtc-udp=block",
		)
	}
	if lat, lon, _, ok := a.geolocationFix(p); ok {
		args = append(args, "--fingerprint-geolocation="+
			strconv.FormatFloat(lat, 'f', 6, 64)+","+strconv.FormatFloat(lon, 'f', 6, 64))
	}
	if p.EngineMeta.CanvasNoise {
		// Deterministic, profile-seeded, alpha-preserving; see patch 0120.
		args = append(args, "--fingerprint-noise")
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
	bin := filepath.Join(a.dir(r.Version), filepath.FromSlash(r.Bin))
	return engine.LaunchSpec{Binary: bin, Args: args, Env: env, EnvUnset: []string{
		"GOOGLE_API_KEY", "GOOGLE_DEFAULT_CLIENT_ID", "GOOGLE_DEFAULT_CLIENT_SECRET",
	}, CDPPipe: rt.CDPPipe, Warning: strings.Join(warn, "; ")}, nil
}
