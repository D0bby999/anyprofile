// SPDX-License-Identifier: AGPL-3.0-or-later

package pro

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

// The public half of the licence signing key. It is intentionally separate
// from publicKey (packs) and the app update key. Production builds should
// replace this with the live key before shipping; the plan records rotation.
var licensePublicKey = mustKey("uyAErxBEjJ2r0FvaLLs60Zo4qXYSsKeGCdPGlT8pUVA=")

// ErrUnsupported says this server predates /v1/activate or /v1/license/refresh;
// the app then uses the legacy key path for one release.
var ErrUnsupported = errors.New("licence endpoint not supported")

// Certificate is the verified payload of a Pro licence JWS.
type Certificate struct {
	Token      string   `json:"-"`
	Issuer     string   `json:"iss"`
	Audience   string   `json:"aud"`
	Subject    string   `json:"sub"`
	JTI        string   `json:"jti"`
	IssuedAt   int64    `json:"iat"`
	NotBefore  int64    `json:"nbf"`
	ExpiresAt  int64    `json:"exp"`
	LicenseID  string   `json:"license_id"`
	Source     string   `json:"source"`
	Kind       string   `json:"kind"`
	Status     string   `json:"status"`
	Features   []string `json:"features"`
	MaxDevices int      `json:"max_devices"`
	Device     struct {
		Alg    string `json:"alg"`
		Pubkey string `json:"pubkey"`
	} `json:"device"`
	PaidThrough      int64 `json:"paid_through"`
	LicenseExpiresAt int64 `json:"license_expires_at"`
	GraceUntil       int64 `json:"grace_until"`
	UpdatesThrough   int64 `json:"updates_through"`
	RefreshAfter     int64 `json:"refresh_after"`
}

// Has reports whether the certificate grants feature.
func (c *Certificate) Has(feature string) bool {
	for _, f := range c.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// Allowed says Pro features may run now: until grace_until, or forever for a
// lifetime certificate that has no expiry.
func (c *Certificate) Allowed(now time.Time) bool {
	if c.Status == "revoked" || c.Status == "pending" {
		return false
	}
	if c.Kind == "lifetime" && c.GraceUntil == 0 {
		return true
	}
	return c.GraceUntil > 0 && now.Unix() <= c.GraceUntil
}

// UpdatesAllowed says a newer pack/runtime may be downloaded now.
func (c *Certificate) UpdatesAllowed(now time.Time) bool {
	if !c.Allowed(now) {
		return false
	}
	return c.UpdatesThrough == 0 || now.Unix() <= c.UpdatesThrough
}

// Offline says the certificate itself has expired but its grace has not.
func (c *Certificate) Offline(now time.Time) bool {
	return c.ExpiresAt > 0 && now.Unix() > c.ExpiresAt && c.Allowed(now)
}

// VerifyCertificate checks a compact JWS, its algorithm/key id, issuer,
// audience and device binding. It accepts a certificate past exp while still
// inside grace_until; the app must not require a fresh certificate to work.
func VerifyCertificate(keys map[string]ed25519.PublicKey, token, issuer, audience, devicePub string, now time.Time) (*Certificate, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, apperr.New("pro_bad_certificate", "the Pro certificate is not a signed token")
	}
	head, err := decodeURL(parts[0])
	if err != nil {
		return nil, apperr.New("pro_bad_certificate", "the Pro certificate header is not base64url")
	}
	body, err := decodeURL(parts[1])
	if err != nil {
		return nil, apperr.New("pro_bad_certificate", "the Pro certificate payload is not base64url")
	}
	sig, err := decodeURL(parts[2])
	if err != nil {
		return nil, apperr.New("pro_bad_certificate", "the Pro certificate signature is not base64url")
	}
	var h struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(head, &h); err != nil || h.Alg != "EdDSA" || h.Typ != "anyprofile-license+jwt" {
		return nil, apperr.New("pro_bad_certificate", "the Pro certificate algorithm is not accepted")
	}
	pub := keys[h.Kid]
	if pub == nil {
		return nil, apperr.New("pro_bad_certificate", "the Pro certificate key is not known")
	}
	if !ed25519.Verify(pub, []byte(parts[0]+"."+parts[1]), sig) {
		return nil, apperr.New("pro_bad_certificate", "the Pro certificate signature is not valid")
	}
	var c Certificate
	if err := json.Unmarshal(body, &c); err != nil {
		return nil, apperr.New("pro_bad_certificate", "the Pro certificate payload is not JSON")
	}
	c.Token = token
	if issuer != "" && c.Issuer != issuer {
		return nil, apperr.New("pro_bad_certificate", "the Pro certificate issuer is not right")
	}
	if audience != "" && c.Audience != audience {
		return nil, apperr.New("pro_bad_certificate", "the Pro certificate audience is not right")
	}
	if c.Device.Pubkey == "" || c.Device.Pubkey != devicePub {
		return nil, apperr.New("pro_device_mismatch", "the Pro certificate belongs to another device")
	}
	skew := int64(5 * time.Minute / time.Second)
	if c.NotBefore > now.Unix()+skew || c.IssuedAt > now.Unix()+skew {
		return nil, apperr.New("pro_bad_certificate", "the Pro certificate is from the future")
	}
	// An expired certificate is still a valid signed statement; the app shows
	// "expired" and the gate refuses Pro. Rejecting it here would hide the
	// status and make a lapsed customer look like a corrupt licence.
	if !c.Allowed(now) && c.Status != "revoked" {
		c.Status = "expired"
	}
	return &c, nil
}

func decodeURL(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// NewDeviceSeed makes the private half of a device key. The seed is stored in
// the vault; the server only ever sees the public half.
func NewDeviceSeed() ([]byte, error) {
	b := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// DevicePublicKey is the raw base64 public half of seed.
func DevicePublicKey(seed []byte) (string, error) {
	if len(seed) != ed25519.SeedSize {
		return "", fmt.Errorf("device seed is %d bytes, want %d", len(seed), ed25519.SeedSize)
	}
	return base64.StdEncoding.EncodeToString(ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)), nil
}

// DeviceProof is the signature the app sends with refresh/pack requests.
func DeviceProof(seed []byte, licenseID, devicePub string, at int64) (string, error) {
	if len(seed) != ed25519.SeedSize {
		return "", fmt.Errorf("device seed is %d bytes, want %d", len(seed), ed25519.SeedSize)
	}
	msg := fmt.Sprintf("%s\n%s\n%d", licenseID, devicePub, at)
	return base64.StdEncoding.EncodeToString(ed25519.Sign(ed25519.NewKeyFromSeed(seed), []byte(msg))), nil
}

// LicenseView is the server's short status beside the certificate.
type LicenseView struct {
	ID               string   `json:"id"`
	Kind             string   `json:"kind"`
	Source           string   `json:"source"`
	Status           string   `json:"status"`
	Features         []string `json:"features"`
	MaxDevices       int      `json:"max_devices"`
	PaidThrough      int64    `json:"paid_through"`
	LicenseExpiresAt int64    `json:"license_expires_at"`
	GraceUntil       int64    `json:"grace_until"`
	UpdatesThrough   int64    `json:"updates_through"`
	Devices          int      `json:"devices"`
	Email            string   `json:"email"`
}

// ActivateRequest claims one device slot with the bearer key.
type ActivateRequest struct {
	Key          string `json:"key"`
	InstallID    string `json:"install_id"`
	DevicePubkey string `json:"device_pubkey"`
	App          string `json:"app"`
	Platform     string `json:"platform"`
	Name         string `json:"name,omitempty"`
}

// ActivateAnswer is the signed certificate plus the licence view.
type ActivateAnswer struct {
	Cert    string      `json:"cert"`
	License LicenseView `json:"license"`
}

// Activate exchanges a key for a device-bound certificate.
func (c Client) Activate(ctx context.Context, in ActivateRequest) (ActivateAnswer, error) {
	var out ActivateAnswer
	status, err := c.do(ctx, http.MethodPost, "/v1/activate", in, &out)
	if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
		return out, ErrUnsupported
	}
	return out, err
}

// RefreshRequest asks for a fresh certificate; sig covers license_id,
// device_pubkey and ts.
type RefreshRequest struct {
	LicenseID    string `json:"license_id"`
	DevicePubkey string `json:"device_pubkey"`
	TS           int64  `json:"ts"`
	Sig          string `json:"sig"`
}

// RefreshAnswer is a fresh certificate.
type RefreshAnswer struct {
	Cert    string      `json:"cert"`
	License LicenseView `json:"license"`
}

// Refresh renews the certificate without sending the bearer key.
func (c Client) Refresh(ctx context.Context, in RefreshRequest) (RefreshAnswer, error) {
	var out RefreshAnswer
	status, err := c.do(ctx, http.MethodPost, "/v1/license/refresh", in, &out)
	if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
		return out, ErrUnsupported
	}
	return out, err
}

// GiftRequest redeems a promo or paid gift code.
type GiftRequest struct {
	Code             string `json:"code"`
	InstallID        string `json:"install_id"`
	DevicePubkey     string `json:"device_pubkey"`
	App              string `json:"app"`
	Platform         string `json:"platform"`
	Name             string `json:"name,omitempty"`
	CurrentLicenseID string `json:"current_license_id,omitempty"`
}

// GiftAnswer is the certificate, licence view and, for a new licence, its key.
type GiftAnswer struct {
	Cert    string      `json:"cert"`
	License LicenseView `json:"license"`
	Key     string      `json:"key,omitempty"`
}

// RedeemGift exchanges a code for a certificate.
func (c Client) RedeemGift(ctx context.Context, in GiftRequest) (GiftAnswer, error) {
	var out GiftAnswer
	status, err := c.do(ctx, http.MethodPost, "/v1/gift/redeem", in, &out)
	if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
		return out, ErrUnsupported
	}
	return out, err
}

// CertAsk asks for the newest pack with a device-signed request.
type CertAsk struct {
	LicenseID    string `json:"license_id"`
	DevicePubkey string `json:"device_pubkey"`
	TS           int64  `json:"ts"`
	Sig          string `json:"sig"`
	Have         string `json:"have"`
	App          string `json:"app"`
	Platform     string `json:"platform"`
}

// CheckCert is Check for a certificate holder.
func (c Client) CheckCert(ctx context.Context, a CertAsk) (Answer, error) {
	var out Answer
	if _, err := c.do(ctx, http.MethodPost, "/v1/pack", a, &out); err != nil {
		return out, err
	}
	ok := out.Runtimes[:0]
	for _, r := range out.Runtimes {
		if c.verifyRuntime(r) {
			ok = append(ok, r)
		}
	}
	out.Runtimes = ok
	return out, nil
}

// LicenseKey returns the embedded licence verification key; tests override it.
func (c Client) LicenseKey() ed25519.PublicKey {
	if c.LicenseVerifyKey != nil {
		return c.LicenseVerifyKey
	}
	return licensePublicKey
}

// VerifyCertificate verifies token for this client's licence key.
func (c Client) VerifyCertificate(token, issuer, audience, devicePub string, now time.Time) (*Certificate, error) {
	keys := map[string]ed25519.PublicKey{}
	// The kid travels in the token; this app knows the current and previous
	// ids. Keep the list explicit so a token cannot choose an arbitrary key.
	for _, kid := range []string{"lic-test", "lic-2026-10", "lic-2027-01"} {
		keys[kid] = c.LicenseKey()
	}
	return VerifyCertificate(keys, token, issuer, audience, devicePub, now)
}

// LicenseIssuer/Audience are the values the server puts in a certificate.
const (
	LicenseIssuer   = "https://pro.anytools.world"
	LicenseAudience = "anyprofile-desktop"
)

// DeviceInfo is one machine on a licence.
type DeviceInfo struct {
	InstallID string `json:"install_id"`
	Name      string `json:"name"`
	Platform  string `json:"platform"`
	LastSeen  int64  `json:"last_seen"`
	Current   bool   `json:"current"`
}

// DeviceProof is the same signature used for pack/refresh.
type DeviceProofRequest struct {
	LicenseID    string `json:"license_id"`
	DevicePubkey string `json:"device_pubkey"`
	TS           int64  `json:"ts"`
	Sig          string `json:"sig"`
}

// Devices lists the machines on this licence.
func (c Client) Devices(ctx context.Context, in DeviceProofRequest) ([]DeviceInfo, error) {
	var out struct {
		Devices []DeviceInfo `json:"devices"`
	}
	status, err := c.do(ctx, http.MethodPost, "/v1/license/devices", in, &out)
	if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
		return nil, ErrUnsupported
	}
	return out.Devices, err
}

// RemoveDeviceV2 frees one machine slot for this licence.
func (c Client) RemoveDeviceV2(ctx context.Context, in DeviceProofRequest, installID string) error {
	status, err := c.do(ctx, http.MethodPost, "/v1/license/device/remove", struct {
		DeviceProofRequest
		InstallID string `json:"install_id"`
	}{in, installID}, nil)
	if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
		return ErrUnsupported
	}
	return err
}
