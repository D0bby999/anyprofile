// SPDX-License-Identifier: AGPL-3.0-or-later

package anybrowser

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/store"
)

// seedDraws bounds how many seeds a new profile tries before falling back to
// the host's own machine. On an 8-core host only the chips that shipped with 8
// cores are servable, so a few draws are expected; 24 leaves the odds of a
// false fallback well under one percent.
const seedDraws = 24

// errEngineMissing means the binary itself is not there, which is the only case
// where profile creation keeps the old host-machine fallback. Any other failure
// is refused: silently pinning the host machine is what produced impossible
// machines in the first place.
var errEngineMissing = errors.New("engine binary is not installed")

// canvasNoiseForNewProfiles is the policy switch for the profile-seeded canvas
// readback noise. It is off until the canvas decision (route A) is approved;
// profiles created before a change keep their stored value, so their pixels do
// not move.
const canvasNoiseForNewProfiles = false

// Composition is the part of --fingerprint-explain that decides whether this
// host can serve a seed.
type Composition struct {
	Servable bool
	Cores    int // 0 when the engine composed none
	MemGiB   int // 0 when the engine composed none
}

var (
	compositionCPU   = regexp.MustCompile(`(?m)^cpu\s+\S+\s+\S+\s+(.+)$`)
	compositionMem   = regexp.MustCompile(`(?m)^memory\s+\S+\s+\S+\s+(.+)$`)
	compositionCores = regexp.MustCompile(`cores-(\d+)`)
	compositionGiB   = regexp.MustCompile(`gib-(\d+)`)
)

// ParseComposition reads the cpu (and memory) rows of --fingerprint-explain.
//
//	"cpu  dispersion  catalogue-value  cores-8"        -> servable, 8 cores
//	"cpu  host-inherited  host-inherited  (inherited)" -> not servable
//
// The engine leaves a surface "inherited" when no catalogue option fits this
// host; for cpu that means the claimed chip needs more cores than the machine
// has, and launching would produce a machine that cannot exist.
func ParseComposition(out string) (Composition, error) {
	cpu := compositionCPU.FindStringSubmatch(out)
	if cpu == nil {
		return Composition{}, errors.New("explain output has no cpu line")
	}
	value := cpu[1]
	if strings.Contains(value, "inherited") {
		return Composition{}, nil
	}
	cores := compositionCores.FindStringSubmatch(value)
	if cores == nil {
		return Composition{}, fmt.Errorf("unrecognised cpu composition %q", value)
	}
	n, err := strconv.Atoi(cores[1])
	if err != nil || n <= 0 {
		return Composition{}, fmt.Errorf("unrecognised cpu core count %q", cores[1])
	}
	c := Composition{Servable: true, Cores: n}
	if mem := compositionMem.FindStringSubmatch(out); mem != nil {
		if gib := compositionGiB.FindStringSubmatch(mem[1]); gib != nil {
			if g, err := strconv.Atoi(gib[1]); err == nil && g > 0 {
				c.MemGiB = g
			}
		}
	}
	return c, nil
}

// Complete reports whether the machine was composed in full: a servable chip
// and a memory this host can serve. A chip left with only inherited memory
// would still be pinned to the host's memory, which can describe a machine that
// never shipped in that configuration (an M1 Pro with 8 GiB, say).
func (c Composition) Complete() bool { return c.Servable && c.MemGiB > 0 }

// drawServable draws seeds until compose reports one this host can serve. It
// stops at attempts, or at the first compose error (the engine is not
// installed, so nothing can be checked), and reports whether it found one.
func drawServable(draw func() string, compose func(seed string) (Composition, error), attempts int) (string, Composition, bool, error) {
	for i := 0; i < attempts; i++ {
		seed := draw()
		if seed == "" {
			return "", Composition{}, false, nil
		}
		c, err := compose(seed)
		if err != nil {
			return "", Composition{}, false, err
		}
		if c.Servable {
			return seed, c, true, nil
		}
	}
	return "", Composition{}, false, nil
}

// ComposeIdentity draws a seed the installed release can serve in full and
// pins the machine it composes. A seed whose chip needs more cores than this
// host has would otherwise come back with the host's core count and describe a
// machine that never existed (an M4 with 8 cores, say). When the engine cannot
// be run — not installed yet — the host's own machine is used, as before.
func (a *AnyBrowser) ComposeIdentity(ctx context.Context, version string, roll uint64) (engine.Identity, error) {
	cores, mem := a.hostInfo()
	host := store.EngineMeta{HWC: cores, MemGiB: mem, CanvasNoise: canvasNoiseForNewProfiles}
	if !a.composeSeeds {
		return engine.Identity{PersonaID: a.persona, Platform: platform, Meta: host}, nil
	}
	seed, comp, ok, err := drawServable(randomSeed, func(seed string) (Composition, error) {
		c, err := a.compose(ctx, version, seed)
		if err != nil {
			return c, err
		}
		c.Servable = c.Complete()
		return c, nil
	}, seedDraws)
	if err := ctx.Err(); err != nil {
		return engine.Identity{}, err
	}
	if err != nil {
		if errors.Is(err, errEngineMissing) {
			return engine.Identity{}, errors.New("AnyBrowser is not installed; install it before creating a profile")
		}
		return engine.Identity{}, fmt.Errorf("compose profile identity: %w", err)
	}
	if !ok {
		return engine.Identity{}, errors.New("no machine this host can serve was found; refusing to pin an impossible one")
	}
	meta := store.EngineMeta{HWC: comp.Cores, MemGiB: comp.MemGiB, CanvasNoise: canvasNoiseForNewProfiles}
	if meta.HWC <= 0 {
		meta.HWC = cores
	}
	if meta.MemGiB <= 0 {
		meta.MemGiB = mem
	}
	return engine.Identity{PersonaID: a.persona, Platform: platform, Meta: meta, SeedHex: seed}, nil
}

// compose runs the release's own explain mode on a throwaway profile. It never
// opens a window: the binary prints the composition and exits, and --disable-gpu
// keeps it away from the host's GPU and from a crash-restore state.
func (a *AnyBrowser) compose(ctx context.Context, version, seed string) (Composition, error) {
	r, err := a.release(version)
	if err != nil {
		return Composition{}, err
	}
	bin := filepath.Join(a.dir(version), filepath.FromSlash(r.Bin))
	if _, err := os.Stat(bin); err != nil {
		return Composition{}, fmt.Errorf("%w: %s", errEngineMissing, bin)
	}
	dir, err := os.MkdirTemp("", "anyprofile-compose-")
	if err != nil {
		return Composition{}, err
	}
	defer os.RemoveAll(dir)
	if err := os.Chmod(dir, 0o700); err != nil {
		return Composition{}, err
	}
	runCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, bin,
		"--fingerprint="+seed,
		"--fingerprint-platform="+platform,
		"--fingerprint-explain",
		"--disable-gpu",
		"--no-first-run",
		"--hide-crash-restore-bubble",
		"--user-data-dir="+dir,
	)
	cmd.WaitDelay = 5 * time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return Composition{}, fmt.Errorf("fingerprint explain: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return ParseComposition(string(out))
}

// randomSeed is the 16-hex-digit seed a profile is composed from.
func randomSeed() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
