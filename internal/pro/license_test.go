// SPDX-License-Identifier: AGPL-3.0-or-later

package pro

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestVerifyCertificateAndDeviceProof(t *testing.T) {
	seed, err := base64.StdEncoding.DecodeString("5sCoNuzEee7w9kutQd5PtMI1GME/GiWwXNb3QGLd5dg=")
	if err != nil {
		t.Fatal(err)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	now := time.Unix(1_800_000_000, 0)
	header := `{"alg":"EdDSA","kid":"lic-test","typ":"anyprofile-license+jwt"}`
	claims := map[string]any{
		"iss": LicenseIssuer, "aud": LicenseAudience, "sub": "lic_1", "jti": "cert_1",
		"iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(7 * 24 * time.Hour).Unix(),
		"license_id": "lic_1", "source": "admin", "kind": "lifetime", "status": "active",
		"features": []string{"mobile", "recipes"}, "max_devices": 3,
		"device": map[string]string{"alg": "Ed25519", "pubkey": pubB64},
		"paid_through": 0, "license_expires_at": 0,
		"grace_until": now.Add(5 * 365 * 24 * time.Hour).Unix(), "updates_through": 0,
		"refresh_after": now.Add(24 * time.Hour).Unix(),
	}
	body, _ := json.Marshal(claims)
	h := base64.RawURLEncoding.EncodeToString([]byte(header))
	p := base64.RawURLEncoding.EncodeToString(body)
	sig := ed25519.Sign(priv, []byte(h+"."+p))
	token := h + "." + p + "." + base64.RawURLEncoding.EncodeToString(sig)

	c, err := VerifyCertificate(map[string]ed25519.PublicKey{"lic-test": pub}, token, LicenseIssuer, LicenseAudience, pubB64, now)
	if err != nil {
		t.Fatal(err)
	}
	if c.Kind != "lifetime" || !c.Has("mobile") || c.Has("nope") || !c.Allowed(now) || !c.UpdatesAllowed(now) {
		t.Fatalf("certificate %+v", c)
	}
	if _, err := VerifyCertificate(map[string]ed25519.PublicKey{"lic-test": pub}, token, LicenseIssuer, LicenseAudience, "other", now); err == nil {
		t.Fatal("a certificate for another device was accepted")
	}
	tampered := h + "." + p + "." + base64.RawURLEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
	if _, err := VerifyCertificate(map[string]ed25519.PublicKey{"lic-test": pub}, tampered, LicenseIssuer, LicenseAudience, pubB64, now); err == nil {
		t.Fatal("a tampered certificate was accepted")
	}
	expired, err := VerifyCertificate(map[string]ed25519.PublicKey{"lic-test": pub}, token, LicenseIssuer, LicenseAudience, pubB64, now.Add(10*365*24*time.Hour))
	if err != nil || expired.Status != "expired" {
		t.Fatalf("expired certificate: %+v %v", expired, err)
	}

	// The same seed must produce the device signature the Worker verifies.
	proof, err := DeviceProof(seed, "lic_1", pubB64, 123)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.StdEncoding.DecodeString(proof)
	if !ed25519.Verify(pub, []byte("lic_1\n"+pubB64+"\n123"), raw) {
		t.Fatal("device proof does not verify")
	}
}
