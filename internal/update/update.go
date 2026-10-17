// SPDX-License-Identifier: AGPL-3.0-or-later

// Package update finds, downloads and verifies new AnyProfile releases, and
// hands a verified one to the OS-specific installer (apply_<os>.go).
//
// A release is a latest.json manifest beside one file per platform. Each file
// is signed with the release key (Ed25519); the signature covers the version,
// the platform and the file's SHA-256, so neither a changed file nor an old
// release presented as a newer one passes. The public key is built in.
package update

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/D0bby999/anyprofile/internal/engine"
)

// Current is this build's version. Release builds set it from the tag:
// -ldflags "-X github.com/D0bby999/anyprofile/internal/update.Current=0.2.0".
var Current = "0.1.0"

// ManifestURL is where the newest release is described. EnvManifestURL
// overrides it, for tests and acceptance runs.
var ManifestURL = "https://github.com/D0bby999/anyprofile/releases/latest/download/latest.json"

const EnvManifestURL = "ANYPROFILE_UPDATE_URL"

// publicKey verifies release signatures. Its private half is made by
// `go run ./tools/release keygen` and never leaves the release machine's
// Keychain and the CI secret UPDATE_SIGNING_KEY.
var publicKey = mustKey("f+DenBOqPGKEMgvdeLCZuI8d4UDitMZdbQ/zpQO5Fc4=")

func mustKey(b64 string) ed25519.PublicKey {
	k, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(k) != ed25519.PublicKeySize {
		return nil // no key built in: every signature fails
	}
	return ed25519.PublicKey(k)
}

// Asset is one platform's file.
type Asset struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Sig    string `json:"sig"` // base64 Ed25519 over SignedMessage
}

// Manifest is latest.json.
type Manifest struct {
	Version string           `json:"version"`
	Date    string           `json:"date"`
	Notes   string           `json:"notes"`
	Assets  map[string]Asset `json:"assets"`
	// Engines lists engine releases tested with AnyProfile, so a new engine
	// version needs no new app. EnginesSig signs its compact JSON (see
	// EnginesMessage); both are empty when the release lists none.
	Engines    json.RawMessage `json:"engines,omitempty"`
	EnginesSig string          `json:"enginesSig,omitempty"`
}

// EnginesMessage is what the engine list's signature covers: its JSON with
// all insignificant space removed, so indenting latest.json breaks nothing.
func EnginesMessage(raw []byte) ([]byte, error) {
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return nil, err
	}
	return append([]byte("anyprofile-engines\n"), b.Bytes()...), nil
}

var ErrBadEnginesSignature = errors.New("the engine list is not signed by AnyProfile's release key")

// VerifyEngines checks the engine list's signature and reads it.
func VerifyEngines(raw []byte, sig string) ([]engine.Release, error) {
	return verifyEngines(publicKey, raw, sig)
}

func verifyEngines(pub ed25519.PublicKey, raw []byte, sig string) ([]engine.Release, error) {
	msg, err := EnginesMessage(raw)
	if err != nil {
		return nil, fmt.Errorf("read engine list: %w", err)
	}
	s, err := base64.StdEncoding.DecodeString(sig)
	if err != nil || len(pub) != ed25519.PublicKeySize || !ed25519.Verify(pub, msg, s) {
		return nil, ErrBadEnginesSignature
	}
	var rs []engine.Release
	if err := json.Unmarshal(raw, &rs); err != nil {
		return nil, fmt.Errorf("read engine list: %w", err)
	}
	return rs, nil
}

// Platform names this build's asset, e.g. "darwin-arm64" or "windows-amd64".
func Platform() string { return runtime.GOOS + "-" + runtime.GOARCH }

// SignedMessage is what a release signature covers.
func SignedMessage(version, platform, sha256hex string) []byte {
	return []byte("anyprofile-update\n" + version + "\n" + platform + "\n" + sha256hex)
}

var versionRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// Newer reports whether version a is above b. Both are MAJOR.MINOR.PATCH;
// anything else is never newer.
func Newer(a, b string) bool {
	pa, pb := parts(a), parts(b)
	if pa == nil || pb == nil {
		return false
	}
	for i := range 3 {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

func parts(v string) []int {
	v, _, _ = strings.Cut(strings.TrimPrefix(v, "v"), "-")
	if !versionRe.MatchString(v) {
		return nil
	}
	out := make([]int, 3)
	for i, s := range strings.Split(v, ".") {
		out[i], _ = strconv.Atoi(s)
	}
	return out
}

func manifestURL() string {
	if v := os.Getenv(EnvManifestURL); v != "" {
		return v
	}
	return ManifestURL
}

// Fetch reads the newest release's manifest.
func Fetch(ctx context.Context, c *http.Client) (Manifest, error) {
	var m Manifest
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL(), nil)
	if err != nil {
		return m, err
	}
	req.Header.Set("User-Agent", "AnyProfile/"+Current)
	resp, err := c.Do(req)
	if err != nil {
		return m, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return m, fmt.Errorf("update server answered %s", resp.Status)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&m); err != nil {
		return m, fmt.Errorf("read update manifest: %w", err)
	}
	if parts(m.Version) == nil {
		return m, fmt.Errorf("update manifest has a bad version %q", m.Version)
	}
	return m, nil
}

var ErrNoAsset = errors.New("this release has no file for this computer")

// AssetFor returns the manifest's file for platform, checking its signature
// before anything is downloaded.
func (m Manifest) AssetFor(platform string) (Asset, error) {
	a, ok := m.Assets[platform]
	if !ok {
		return a, ErrNoAsset
	}
	if !strings.HasPrefix(a.URL, "https://") {
		return a, errors.New("update file is not served over https")
	}
	return a, verifySig(publicKey, a, m.Version, platform)
}

var ErrBadSignature = errors.New("the update is not signed by AnyProfile's release key")

func verifySig(pub ed25519.PublicKey, a Asset, version, platform string) error {
	sig, err := base64.StdEncoding.DecodeString(a.Sig)
	if err != nil || len(pub) != ed25519.PublicKeySize || len(a.SHA256) != 64 ||
		!ed25519.Verify(pub, SignedMessage(version, platform, strings.ToLower(a.SHA256)), sig) {
		return ErrBadSignature
	}
	return nil
}

// Progress reports downloaded and total bytes (total is -1 when unknown).
type Progress func(done, total int64)

// Download fetches a verified asset into dir and returns the file's path.
// The file is removed again unless its SHA-256 matches the signed one.
func Download(ctx context.Context, c *http.Client, a Asset, dir string, progress Progress) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	name := filepath.Base(strings.SplitN(a.URL, "?", 2)[0])
	if name == "" || name == "." || name == "/" || strings.ContainsAny(name, `\:`) {
		name = "update.bin"
	}
	dst := filepath.Join(dir, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "AnyProfile/"+Current)
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download answered %s", resp.Status)
	}
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), &counter{r: resp.Body, total: resp.ContentLength, progress: progress})
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), a.SHA256) {
		err = errors.New("the downloaded update does not match its signed checksum")
	}
	if err != nil {
		os.Remove(dst)
		return "", err
	}
	return dst, nil
}

type counter struct {
	r        io.Reader
	done     int64
	total    int64
	progress Progress
}

func (c *counter) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.done += int64(n)
	if c.progress != nil && n > 0 {
		c.progress(c.done, c.total)
	}
	return n, err
}
