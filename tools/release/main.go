// SPDX-License-Identifier: AGPL-3.0-or-later

// release signs AnyProfile releases for the in-app updater.
//
//	go run ./tools/release keygen     make the release key; prints its public half
//	go run ./tools/release pubkey     print the public half again
//	go run ./tools/release export-key | gh secret set UPDATE_SIGNING_KEY
//	                                   hand the key to CI; refuses a terminal
//	go run ./tools/release manifest -version 0.2.0 -base-url URL [-notes-file F] [-engines engines.json] \
//	    [-o latest.json] darwin-arm64=AnyProfile-macos-arm64.zip windows-amd64=AnyProfile-amd64-installer.exe
//	go run ./tools/release engines -manifest latest.json engines.json
//	                                   sign engines.json into an existing latest.json, so a tested
//	                                   engine version reaches users without an app release
//
// The private key comes from UPDATE_SIGNING_KEY (base64 Ed25519 seed; CI) or,
// on a Mac, from the Keychain item keygen made. It is never printed.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/update"
)

func main() {
	if len(os.Args) < 2 {
		fail(errors.New("usage: release keygen | pubkey | manifest ..."))
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = keygen()
	case "pubkey":
		var k ed25519.PrivateKey
		if k, err = signingKey(); err == nil {
			fmt.Println(base64.StdEncoding.EncodeToString(k.Public().(ed25519.PublicKey)))
		}
	case "export-key":
		err = exportKey()
	case "manifest":
		err = manifest(os.Args[2:])
	case "engines":
		err = engines(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "release:", err)
	os.Exit(1)
}

func keygen() error {
	if _, err := loadSeed(); err == nil {
		return errors.New("a release key already exists; replacing it would strand every installed copy")
	}
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return err
	}
	if err := saveSeed(seed); err != nil {
		return err
	}
	pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	fmt.Println(base64.StdEncoding.EncodeToString(pub))
	return nil
}

// exportKey writes the key for a pipe, never to a terminal or a log.
func exportKey() error {
	if fi, err := os.Stdout.Stat(); err != nil || fi.Mode()&os.ModeCharDevice != 0 {
		return errors.New("export-key only writes into a pipe, e.g. | gh secret set UPDATE_SIGNING_KEY")
	}
	seed, err := loadSeed()
	if err != nil {
		return err
	}
	_, err = os.Stdout.WriteString(base64.StdEncoding.EncodeToString(seed))
	return err
}

func signingKey() (ed25519.PrivateKey, error) {
	if v := os.Getenv("UPDATE_SIGNING_KEY"); v != "" {
		seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(v))
		if err != nil || len(seed) != ed25519.SeedSize {
			return nil, errors.New("UPDATE_SIGNING_KEY is not a base64 Ed25519 seed")
		}
		return ed25519.NewKeyFromSeed(seed), nil
	}
	seed, err := loadSeed()
	if err != nil {
		return nil, fmt.Errorf("no release key (set UPDATE_SIGNING_KEY or run keygen): %w", err)
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

func manifest(args []string) error {
	fs := flag.NewFlagSet("manifest", flag.ExitOnError)
	version := fs.String("version", "", "release version, MAJOR.MINOR.PATCH")
	baseURL := fs.String("base-url", "", "https URL the files are served under")
	notesFile := fs.String("notes-file", "", "release notes (plain text)")
	enginesFile := fs.String("engines", "", "engine releases to sign in (engines.json)")
	out := fs.String("o", "", "write here instead of stdout")
	fs.Parse(args)
	if !update.Newer(*version, "0.0.0") || !strings.HasPrefix(*baseURL, "https://") || fs.NArg() == 0 {
		return errors.New("manifest needs -version X.Y.Z, an https -base-url and platform=file pairs")
	}
	key, err := signingKey()
	if err != nil {
		return err
	}
	m := update.Manifest{Version: *version, Date: time.Now().UTC().Format("2006-01-02"), Assets: map[string]update.Asset{}}
	if *notesFile != "" {
		b, err := os.ReadFile(*notesFile)
		if err != nil {
			return err
		}
		m.Notes = strings.TrimSpace(string(b))
	}
	if *enginesFile != "" {
		if m.Engines, m.EnginesSig, err = signEngines(key, *enginesFile); err != nil {
			return err
		}
	}
	for _, pair := range fs.Args() {
		platform, file, ok := strings.Cut(pair, "=")
		if !ok {
			return fmt.Errorf("%q is not platform=file", pair)
		}
		sum, err := fileSHA256(file)
		if err != nil {
			return err
		}
		sig := ed25519.Sign(key, update.SignedMessage(*version, platform, sum))
		m.Assets[platform] = update.Asset{URL: strings.TrimSuffix(*baseURL, "/") + "/" + filepath.Base(file),
			SHA256: sum, Sig: base64.StdEncoding.EncodeToString(sig)}
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	if *out == "" {
		_, err = os.Stdout.Write(append(b, '\n'))
		return err
	}
	return os.WriteFile(*out, append(b, '\n'), 0o644)
}

// engines signs an engine list into an existing latest.json, in place.
func engines(args []string) error {
	fs := flag.NewFlagSet("engines", flag.ExitOnError)
	path := fs.String("manifest", "", "latest.json to update")
	fs.Parse(args)
	if *path == "" || fs.NArg() != 1 {
		return errors.New("engines needs -manifest latest.json and the engines.json to sign")
	}
	b, err := os.ReadFile(*path)
	if err != nil {
		return err
	}
	var m update.Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return fmt.Errorf("read %s: %w", *path, err)
	}
	key, err := signingKey()
	if err != nil {
		return err
	}
	if m.Engines, m.EnginesSig, err = signEngines(key, fs.Arg(0)); err != nil {
		return err
	}
	out, _ := json.MarshalIndent(m, "", "  ")
	return os.WriteFile(*path, append(out, '\n'), 0o644)
}

// signEngines reads engines.json, refuses any release the app would refuse,
// and signs the list's compact form.
func signEngines(key ed25519.PrivateKey, path string) (json.RawMessage, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var rs []engine.Release
	if err := json.Unmarshal(b, &rs); err != nil {
		return nil, "", fmt.Errorf("read %s: %w", path, err)
	}
	for _, r := range rs {
		if err := r.Validate(); err != nil {
			return nil, "", err
		}
	}
	raw, _ := json.Marshal(rs)
	msg, err := update.EnginesMessage(raw)
	if err != nil {
		return nil, "", err
	}
	return raw, base64.StdEncoding.EncodeToString(ed25519.Sign(key, msg)), nil
}

func fileSHA256(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
