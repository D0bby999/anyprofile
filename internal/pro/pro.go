// SPDX-License-Identifier: AGPL-3.0-or-later

// Package pro talks to the AnyProfile Pro license server and keeps the Pro
// pack (recipes and scripts) on disk. Nothing here runs without a key the
// user entered or a purchase the user started: the free app never calls it.
//
// What the server learns: the key, a random install id made on this
// computer, the pack and app versions and the platform. Nothing about
// profiles, proxies or sites.
package pro

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

// DefaultURL is the license server. EnvURL overrides it, for tests and staging.
const (
	DefaultURL = "https://pro.anytools.world"
	EnvURL     = "ANYPROFILE_PRO_URL"
)

// publicKey verifies packs. Its private half is the Pro signing key, apart
// from the app's update key: it can sign a pack, never an app update.
var publicKey = mustKey("lmdkGy4IWL7pDEG7AoHf9tEYDRdgGnPILkC6MJGxWvI=")

func mustKey(b64 string) ed25519.PublicKey {
	k, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(k) != ed25519.PublicKeySize {
		return nil
	}
	return ed25519.PublicKey(k)
}

// SignedMessage is what a pack signature covers; its first line keeps it
// apart from an app update's ("anyprofile-update\n…").
func SignedMessage(version, sha256hex string) []byte {
	return []byte("anyprofile-pro\n" + version + "\n" + sha256hex)
}

// Release is a pack the server offers.
type Release struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	Sig     string `json:"sig"`
	Size    int64  `json:"size"`
	Notes   string `json:"notes"`
	URL     string `json:"url"`
}

// Answer is the server's word on a key: its state and the pack to install, if any.
type Answer struct {
	State       string   `json:"state"` // pending | active | past_due | expired
	PaidThrough int64    `json:"paid_through"`
	Release     *Release `json:"release"`
}

// Ask is what the app tells the server when it checks.
type Ask struct {
	Key       string `json:"key"`
	InstallID string `json:"install_id"`
	Have      string `json:"have"`
	App       string `json:"app"`
	Platform  string `json:"platform"`
}

// ErrNotYet: the purchase this ref started has not been paid (yet).
var ErrNotYet = errors.New("not paid yet")

// maxPack bounds a download; packs are text and a few scripts.
const maxPack = 50 << 20

// Client calls the license server.
type Client struct {
	Base string // no slash at the end
	HTTP *http.Client
	// Key verifies packs; nil means the built-in Pro key. Tests set their own.
	Key ed25519.PublicKey
}

// NewClient uses EnvURL when set, else DefaultURL.
func NewClient() Client {
	base := DefaultURL
	if v := strings.TrimRight(os.Getenv(EnvURL), "/"); v != "" {
		base = v
	}
	return Client{Base: base, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// NewRef makes the one-purchase ref the app polls with.
func NewRef() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// NewInstallID makes this computer's random id; it says nothing about the machine.
func NewInstallID() string {
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// BuyURL is the page that starts a purchase whose key this app will claim with ref.
func (c Client) BuyURL(plan, lang, ref string) string {
	q := url.Values{"plan": {plan}, "lang": {lang}, "ref": {ref}}
	return c.Base + "/buy?" + q.Encode()
}

func (c Client) do(ctx context.Context, method, path string, in, out any) (int, error) {
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+path, body)
	if err != nil {
		return 0, err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return 0, apperr.New("pro_offline", "cannot reach the AnyProfile Pro server: "+err.Error())
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 200 && res.StatusCode < 300 && out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return res.StatusCode, fmt.Errorf("pro server answer: %w", err)
		}
	}
	if res.StatusCode >= 400 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		return res.StatusCode, serverError(res.StatusCode, e.Error)
	}
	return res.StatusCode, nil
}

func serverError(status int, code string) error {
	switch code {
	case "bad_key":
		return apperr.New("pro_bad_key", "this key is not right")
	case "revoked":
		return apperr.New("pro_revoked", "this key was revoked (refund or dispute)")
	case "too_many_devices":
		return apperr.New("pro_too_many_devices", "this key is in use on 3 computers already")
	case "not_yet":
		return ErrNotYet
	case "slow_down":
		return apperr.New("pro_offline", "the Pro server asks to wait a moment")
	}
	return apperr.Newf("pro_offline", "the Pro server answered %d %s", status, code)
}

// Claim takes the key the purchase started with ref, once; ErrNotYet until paid.
func (c Client) Claim(ctx context.Context, ref string) (string, error) {
	var out struct {
		Key string `json:"key"`
	}
	if _, err := c.do(ctx, http.MethodGet, "/v1/claim?ref="+url.QueryEscape(ref), nil, &out); err != nil {
		return "", err
	}
	return out.Key, nil
}

// Check asks for the key's state and a newer pack.
func (c Client) Check(ctx context.Context, a Ask) (Answer, error) {
	var out Answer
	_, err := c.do(ctx, http.MethodPost, "/v1/pack", a, &out)
	return out, err
}

// RemoveDevice frees this computer's place on the key.
func (c Client) RemoveDevice(ctx context.Context, key, installID string) error {
	_, err := c.do(ctx, http.MethodPost, "/v1/device/remove", map[string]string{"key": key, "install_id": installID}, nil)
	return err
}

// Portal is the Stripe page where the subscription is managed.
func (c Client) Portal(ctx context.Context, key string) (string, error) {
	var out struct {
		URL string `json:"url"`
	}
	if _, err := c.do(ctx, http.MethodPost, "/portal", map[string]string{"key": key}, &out); err != nil {
		return "", err
	}
	return out.URL, nil
}

// Download fetches rel into dir and checks its size, checksum and signature.
// It returns the zip's path; nothing unverified is left behind.
func (c Client) Download(ctx context.Context, rel Release, dir string) (string, error) {
	pub := c.Key
	if pub == nil {
		pub = publicKey
	}
	if err := verifySig(pub, rel); err != nil {
		return "", err
	}
	if rel.Size <= 0 || rel.Size > maxPack {
		return "", apperr.Newf("pro_bad_signature", "pack size %d is out of bounds", rel.Size)
	}
	u, err := url.Parse(rel.URL)
	if err != nil || (u.Scheme != "https" && !strings.HasPrefix(rel.URL, c.Base+"/")) {
		return "", apperr.New("pro_bad_signature", "pack link is not https")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rel.URL, nil)
	if err != nil {
		return "", err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", apperr.New("pro_offline", "download the Pro pack: "+err.Error())
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", apperr.Newf("pro_offline", "download the Pro pack: %d", res.StatusCode)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "pack-*.zip")
	if err != nil {
		return "", err
	}
	path := f.Name()
	sum, n, err := copyHashed(f, io.LimitReader(res.Body, rel.Size+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && (n != rel.Size || sum != rel.SHA256) {
		err = apperr.New("pro_bad_signature", "the downloaded pack does not match its signature")
	}
	if err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

func verifySig(pub ed25519.PublicKey, rel Release) error {
	sig, err := base64.StdEncoding.DecodeString(rel.Sig)
	if err != nil || len(pub) != ed25519.PublicKeySize || len(rel.SHA256) != 64 || !versionOK(rel.Version) ||
		!ed25519.Verify(pub, SignedMessage(rel.Version, rel.SHA256), sig) {
		return apperr.New("pro_bad_signature", "the Pro pack's signature is not valid")
	}
	return nil
}

func versionOK(v string) bool {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" || len(p) > 4 || strings.Trim(p, "0123456789") != "" {
			return false
		}
	}
	return true
}
