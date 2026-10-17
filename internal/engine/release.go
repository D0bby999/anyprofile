// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// Release is one build of an engine for one platform, as data: the pinned
// builds are compiled in, and newer ones arrive in a list signed with
// AnyProfile's release key (see internal/update). Paths are relative to the
// version's folder, which holds what the archive unpacks to.
type Release struct {
	Engine   string `json:"engine"`
	Version  string `json:"version"`
	Chromium string `json:"chromium"`
	Platform string `json:"platform"` // GOOS-GOARCH, as HostPlatform
	Date     string `json:"date,omitempty"`
	Notes    string `json:"notes,omitempty"`
	URL      string `json:"url"`    // the archive: .zip or .tar.xz, over https
	SHA256   string `json:"sha256"` // of the archive
	// Files are checked after unpacking and at startup; Bin is one of them.
	Files map[string]string `json:"files"`
	Bin   string            `json:"bin"`
	// App and TeamID: on macOS the bundle's Apple signature must be TeamID's.
	App    string `json:"app,omitempty"`
	TeamID string `json:"teamId,omitempty"`
	// Catalogue names the machines a seed is drawn from. Profiles move to a
	// newer release of the same catalogue on their own: their machine stays.
	Catalogue string `json:"catalogue"`
	// MinApp is the oldest AnyProfile that can run this release.
	MinApp string `json:"minApp,omitempty"`
}

// HostPlatform is this computer's platform, e.g. "darwin-arm64".
func HostPlatform() string { return runtime.GOOS + "-" + runtime.GOARCH }

var (
	idRe       = regexp.MustCompile(`^[a-z][a-z0-9-]{1,30}$`)
	versionRe  = regexp.MustCompile(`^\d+(\.\d+){1,4}$`)
	sha256Re   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	platformRe = regexp.MustCompile(`^(darwin|windows|linux)-(arm64|amd64)$`)
)

// Validate refuses a release that could not be installed safely: a file path
// leaving the version folder, a missing hash, an archive not on https.
func (r Release) Validate() error {
	switch {
	case !idRe.MatchString(r.Engine):
		return fmt.Errorf("release: bad engine %q", r.Engine)
	case !versionRe.MatchString(r.Version):
		return fmt.Errorf("release %s: bad version %q", r.Engine, r.Version)
	case !versionRe.MatchString(r.Chromium):
		return fmt.Errorf("release %s %s: bad chromium %q", r.Engine, r.Version, r.Chromium)
	case !platformRe.MatchString(r.Platform):
		return fmt.Errorf("release %s %s: bad platform %q", r.Engine, r.Version, r.Platform)
	case !strings.HasPrefix(r.URL, "https://"):
		return fmt.Errorf("release %s %s: the archive is not on https", r.Engine, r.Version)
	case !strings.HasSuffix(r.URL, ".zip") && !strings.HasSuffix(r.URL, ".tar.xz"):
		return fmt.Errorf("release %s %s: the archive is neither .zip nor .tar.xz", r.Engine, r.Version)
	case !sha256Re.MatchString(r.SHA256):
		return fmt.Errorf("release %s %s: bad archive sha256", r.Engine, r.Version)
	case r.Files[r.Bin] == "":
		return fmt.Errorf("release %s %s: the binary has no hash", r.Engine, r.Version)
	case r.MinApp != "" && !versionRe.MatchString(r.MinApp):
		return fmt.Errorf("release %s %s: bad minApp %q", r.Engine, r.Version, r.MinApp)
	case r.App != "" && !safeRel(r.App):
		return fmt.Errorf("release %s %s: bad app path", r.Engine, r.Version)
	}
	for p, sum := range r.Files {
		if !safeRel(p) || !sha256Re.MatchString(sum) {
			return fmt.Errorf("release %s %s: bad file %q", r.Engine, r.Version, p)
		}
	}
	return nil
}

// safeRel is a relative, slash-separated path that stays inside its folder.
func safeRel(p string) bool {
	if p == "" || strings.ContainsAny(p, `\:`) || strings.HasPrefix(p, "/") {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

// NewerVersion reports whether dotted version a is above b ("0.10.0" > "0.9.3").
func NewerVersion(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := range max(len(pa), len(pb)) {
		x, y := 0, 0
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x > y
		}
	}
	return false
}

// Catalog is every release AnyProfile knows: the compiled-in ones and the
// last verified signed list.
type Catalog struct {
	app string // this AnyProfile's version; "" accepts any MinApp
	mu  sync.RWMutex
	// builtin wins over a listed release with the same version.
	builtin, listed []Release
}

func NewCatalog(app string, builtin ...Release) *Catalog {
	return &Catalog{app: app, builtin: builtin}
}

// SetListed replaces the releases from the signed list. Invalid ones are
// dropped and reported.
func (c *Catalog) SetListed(rs []Release) error {
	var keep []Release
	var errs []error
	for _, r := range rs {
		if err := r.Validate(); err != nil {
			errs = append(errs, err)
			continue
		}
		keep = append(keep, r)
	}
	c.mu.Lock()
	c.listed = keep
	c.mu.Unlock()
	return errors.Join(errs...)
}

// For lists engine's releases for this computer that this AnyProfile can run,
// newest first.
func (c *Catalog) For(engine string) []Release {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []Release
	seen := map[string]bool{}
	for _, r := range append(slices.Clone(c.builtin), c.listed...) {
		if r.Engine != engine || r.Platform != HostPlatform() || seen[r.Version] {
			continue
		}
		if c.app != "" && r.MinApp != "" && NewerVersion(r.MinApp, c.app) {
			continue
		}
		seen[r.Version] = true
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b Release) int {
		switch {
		case NewerVersion(a.Version, b.Version):
			return -1
		case NewerVersion(b.Version, a.Version):
			return 1
		}
		return 0
	})
	return out
}

// Find is engine's release with version, for this computer.
func (c *Catalog) Find(engine, version string) (Release, bool) {
	for _, r := range c.For(engine) {
		if r.Version == version {
			return r, true
		}
	}
	return Release{}, false
}

// ErrUnknownRelease: no release of that version for this computer.
var ErrUnknownRelease = errors.New("unknown engine version")

// stamp remembers each checked file's size and mtime at its last full
// verification, so startup re-hashes hundreds of MB only after a change.
type stamp struct {
	Size    int64  `json:"size"`
	ModNano int64  `json:"modNano"`
	SHA256  string `json:"sha256"`
}

const stampName = ".verified.json"

// CheckInstalled reports whether dir holds r's files intact. complete adds
// what a release lacks on this OS before the check; it may be nil.
func CheckInstalled(r Release, dir string, complete func(dir string) error) Status {
	st := Status{Version: Version{Engine: r.Engine, Version: r.Version, Chromium: r.Chromium}}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(r.Bin))); err != nil {
		return st
	}
	if complete != nil {
		if err := complete(dir); err != nil {
			st.Problem = err.Error()
			return st
		}
	}
	stamps := readStamps(dir)
	changed := false
	for rel, sum := range r.Files {
		fi, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			st.Problem = "engine files are missing; reinstall"
			return st
		}
		if s, ok := stamps[rel]; ok && s.SHA256 == sum && s.Size == fi.Size() && s.ModNano == fi.ModTime().UnixNano() {
			continue
		}
		if VerifySHA256(filepath.Join(dir, filepath.FromSlash(rel)), sum) != nil {
			st.Problem = "engine files do not match the release; reinstall"
			return st
		}
		stamps[rel] = stamp{Size: fi.Size(), ModNano: fi.ModTime().UnixNano(), SHA256: sum}
		changed = true
	}
	if changed {
		if err := writeStamps(dir, stamps); err != nil {
			st.Problem = err.Error()
			return st
		}
	}
	st.Installed = true
	return st
}

// readStamps reads the stamp file; an older format or a damaged one only
// costs a re-hash.
func readStamps(dir string) map[string]stamp {
	stamps := map[string]stamp{}
	if b, err := os.ReadFile(filepath.Join(dir, stampName)); err != nil || json.Unmarshal(b, &stamps) != nil {
		return map[string]stamp{}
	}
	return stamps
}

func writeStamps(dir string, stamps map[string]stamp) error {
	b, _ := json.Marshal(stamps)
	return os.WriteFile(filepath.Join(dir, stampName), b, 0o600)
}

// InstallRelease downloads r and verifies it in order: archive hash,
// unpacking, every file's hash, then check (the Apple signature, for one) on
// the unpacked tree, which may also add what the release lacks. Nothing is
// moved to dir until all of it passes.
func InstallRelease(ctx context.Context, r Release, dir string, progress Progress, check func(tree string) error) error {
	if err := r.Validate(); err != nil {
		return err
	}
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	staging := filepath.Join(parent, ".staging-"+r.Version)
	os.RemoveAll(staging)
	defer os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return err
	}
	archive := filepath.Join(staging, filepath.Base(r.URL))
	if err := Download(ctx, r.URL, archive, progress); err != nil {
		return err
	}
	if err := VerifySHA256(archive, r.SHA256); err != nil {
		return err
	}
	tree := filepath.Join(staging, "tree")
	extract := ExtractZip
	if strings.HasSuffix(r.URL, ".tar.xz") {
		extract = ExtractTarXZ
	}
	if err := extract(archive, tree); err != nil {
		return fmt.Errorf("extract %s %s: %w", r.Engine, r.Version, err)
	}
	if err := os.Remove(archive); err != nil {
		return err
	}
	for rel, sum := range r.Files {
		if err := VerifySHA256(filepath.Join(tree, filepath.FromSlash(rel)), sum); err != nil {
			return err
		}
	}
	if check != nil {
		if err := check(tree); err != nil {
			return err
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.Rename(tree, dir); err != nil {
		return err
	}
	// Every file was hashed in the tree already: stamp them as they are now.
	stamps := map[string]stamp{}
	for rel, sum := range r.Files {
		fi, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		stamps[rel] = stamp{Size: fi.Size(), ModNano: fi.ModTime().UnixNano(), SHA256: sum}
	}
	return writeStamps(dir, stamps)
}

// CleanStaging removes what an interrupted install left in an engine's
// folder: staging folders and stray archives. Version folders are kept.
func CleanStaging(engineDir string) error {
	stale, _ := filepath.Glob(filepath.Join(engineDir, ".staging-*"))
	for _, pat := range []string{"*.zip", "*.tar.xz", "*.part"} {
		m, _ := filepath.Glob(filepath.Join(engineDir, pat))
		stale = append(stale, m...)
	}
	for _, p := range stale {
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}
	return nil
}
