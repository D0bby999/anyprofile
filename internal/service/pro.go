// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/pro"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/update"
)

// The Pro key, certificate and device seed live in the vault. The vault's key
// is in the Keychain, so no plaintext secret sits in the database or logs.
const (
	proVault        = "anyprofile-pro"
	proAccount      = "license"
	proCertAccount  = "certificate"
	proDeviceAcct   = "device-key"
	proEvery        = 24 * time.Hour
	proLegacyWindow = 30 * 24 * time.Hour
)

// ProStatus is what Settings → Pro shows.
type ProStatus struct {
	Active           bool     `json:"active"`           // a key is entered on this computer
	Key              string   `json:"key"`              // the key with all but its last group hidden
	State            string   `json:"state"`            // pending | active | past_due | expired | revoked; "" before the first check
	Kind             string   `json:"kind"`             // subscription | gift | lifetime | test
	Source           string   `json:"source"`           // stripe | gift | admin | legacy
	Features         []string `json:"features"`         // what the certificate grants
	PaidThrough      string   `json:"paidThrough"`      // RFC 3339, "" when unknown
	LicenseExpiresAt string   `json:"licenseExpiresAt"` // RFC 3339, "" when none
	GraceUntil       string   `json:"graceUntil"`       // RFC 3339, offline use ends
	UpdatesThrough   string   `json:"updatesThrough"`   // RFC 3339, "" means unlimited
	Version          string   `json:"version"`          // the installed pack, "" when none
	Notes            string   `json:"notes"`            // what the installed pack changed
	CheckedAt        string   `json:"checkedAt"`        // RFC 3339, "" when never
	Error            string   `json:"error"`            // the last check's failure, "" when it went well
	Offline          bool     `json:"offline"`          // certificate expired but grace is still open
	Test             bool     `json:"test"`             // a short admin test licence
	Waiting          bool     `json:"waiting"`          // a purchase started here waits for payment (set by the app)
}

type proSaved struct {
	State       string `json:"state"`
	PaidThrough int64  `json:"paidThrough"`
	CheckedAt   string `json:"checkedAt"`
	Notes       string `json:"notes"`
	Error       string `json:"error"`
	// Runtimes are what the server offered at the last check, signatures
	// checked; their links expire within minutes.
	Runtimes []pro.Runtime `json:"runtimes,omitempty"`
}

func (s *Service) proClient() pro.Client {
	if s.Pro.Base != "" {
		return s.Pro
	}
	return pro.NewClient()
}

func (s *Service) proKey() string {
	b, err := s.Secrets.Get(proVault, proAccount)
	if err != nil {
		return ""
	}
	return string(b)
}

func (s *Service) proCertToken() string {
	b, err := s.Secrets.Get(proVault, proCertAccount)
	if err != nil {
		return ""
	}
	return string(b)
}

func (s *Service) saveProCert(token string) error {
	return s.Secrets.Set(proVault, proCertAccount, []byte(token))
}

func (s *Service) deviceSeed() ([]byte, error) {
	b, err := s.Secrets.Get(proVault, proDeviceAcct)
	if err == nil {
		if len(b) != 32 {
			return nil, fmt.Errorf("device seed is %d bytes, want 32", len(b))
		}
		return b, nil
	}
	if !errors.Is(err, secrets.ErrNotFound) {
		return nil, err
	}
	seed, err := pro.NewDeviceSeed()
	if err != nil {
		return nil, err
	}
	if err := s.Secrets.Set(proVault, proDeviceAcct, seed); err != nil {
		return nil, err
	}
	return seed, nil
}

func (s *Service) devicePub() (string, error) {
	seed, err := s.deviceSeed()
	if err != nil {
		return "", err
	}
	return pro.DevicePublicKey(seed)
}

func (s *Service) verifiedCert() (*pro.Certificate, error) {
	token := s.proCertToken()
	if token == "" {
		return nil, apperr.New("pro_no_certificate", "this computer has no Pro certificate")
	}
	pub, err := s.devicePub()
	if err != nil {
		return nil, err
	}
	return s.proClient().VerifyCertificate(token, pro.LicenseIssuer, pro.LicenseAudience, pub, s.now())
}

func (s *Service) acceptCert(token, pub string) (*pro.Certificate, error) {
	cert, err := s.proClient().VerifyCertificate(token, pro.LicenseIssuer, pro.LicenseAudience, pub, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.saveProCert(token); err != nil {
		return nil, err
	}
	return cert, nil
}

func (s *Service) clearCert() {
	_ = s.Secrets.Delete(proVault, proCertAccount)
}

func (s *Service) proSaved() proSaved {
	var p proSaved
	v, _ := s.Store.GetSetting("pro_state")
	_ = json.Unmarshal([]byte(v), &p)
	return p
}

func (s *Service) saveProState(p proSaved) error {
	b, _ := json.Marshal(p)
	return s.Store.SetSetting("pro_state", string(b))
}

// proInstallID is this computer's random id for the license server, made once.
func (s *Service) proInstallID() (string, error) {
	if v, _ := s.Store.GetSetting("pro_install_id"); v != "" {
		return v, nil
	}
	id := pro.NewInstallID()
	return id, s.Store.SetSetting("pro_install_id", id)
}

func maskKey(k string) string {
	i := strings.LastIndex(k, "-")
	if i < 0 || len(k)-i < 3 {
		return "APRO-…"
	}
	return "APRO-•••••-•••••-•••••" + k[i:]
}

func (s *Service) ProStatus() ProStatus {
	key := s.proKey()
	token := s.proCertToken()
	p := s.proSaved()
	st := ProStatus{Active: key != "" || token != "", State: p.State, CheckedAt: p.CheckedAt, Error: p.Error,
		Version: pro.Installed(s.Dirs.Pro())}
	if key != "" {
		st.Key = maskKey(key)
	}
	if st.Version != "" {
		st.Notes = p.Notes
	}
	if p.PaidThrough > 0 {
		st.PaidThrough = time.Unix(p.PaidThrough, 0).UTC().Format(time.RFC3339)
	}
	// The signed certificate is the source of truth when this computer has
	// one. The old unsigned pro_state only fills the gap during migration.
	if cert, err := s.verifiedCert(); err == nil {
		st.State = cert.Status
		if !cert.Allowed(s.now()) && st.State != "revoked" {
			st.State = "expired"
		}
		st.Kind, st.Source, st.Features = cert.Kind, cert.Source, cert.Features
		st.Test = cert.Kind == "test"
		st.Offline = cert.Offline(s.now())
		st.PaidThrough = ts(cert.PaidThrough)
		st.LicenseExpiresAt = ts(cert.LicenseExpiresAt)
		st.GraceUntil = ts(cert.GraceUntil)
		st.UpdatesThrough = ts(cert.UpdatesThrough)
		st.Error = ""
	} else if s.proCertToken() != "" {
		st.State = "expired"
		if st.Error == "" {
			st.Error = err.Error()
		}
	}
	return st
}

func ts(v int64) string {
	if v <= 0 {
		return ""
	}
	return time.Unix(v, 0).UTC().Format(time.RFC3339)
}

func (s *Service) proAsk(key string) (pro.Ask, error) {
	id, err := s.proInstallID()
	return pro.Ask{Key: key, InstallID: id, Have: pro.Installed(s.Dirs.Pro()), App: update.Current,
		Platform: runtime.GOOS + "-" + runtime.GOARCH}, err
}

// activateCertLocked claims a device and stores a verified certificate. It
// returns ErrUnsupported when the server predates /v1/activate; the caller
// then uses the legacy key path for one release.
func (s *Service) activateCertLocked(ctx context.Context, key string) (*pro.Certificate, error) {
	id, err := s.proInstallID()
	if err != nil {
		return nil, err
	}
	seed, err := s.deviceSeed()
	if err != nil {
		return nil, err
	}
	pub, err := pro.DevicePublicKey(seed)
	if err != nil {
		return nil, err
	}
	ans, err := s.proClient().Activate(ctx, pro.ActivateRequest{
		Key: key, InstallID: id, DevicePubkey: pub,
		App: update.Current, Platform: runtime.GOOS + "-" + runtime.GOARCH,
	})
	if err != nil {
		return nil, err
	}
	cert, err := s.acceptCert(ans.Cert, pub)
	if err != nil {
		return nil, err
	}
	if err := s.Secrets.Set(proVault, proAccount, []byte(key)); err != nil {
		s.clearCert()
		return nil, err
	}
	return cert, nil
}

// ProActivate checks a key with the server and, when it is good, keeps it
// and installs the pack it may have. A key the server refuses is not kept.
func (s *Service) ProActivate(ctx context.Context, key string) (ProStatus, error) {
	key = strings.ToUpper(strings.TrimSpace(key))
	if key == "" {
		return s.ProStatus(), apperr.New("pro_bad_key", "enter a key")
	}
	s.proMu.Lock()
	_, err := s.activateCertLocked(ctx, key)
	if errors.Is(err, pro.ErrUnsupported) {
		// Old server: keep the proven key path until it is upgraded.
		ask, aerr := s.proAsk(key)
		if aerr != nil {
			s.proMu.Unlock()
			return s.ProStatus(), aerr
		}
		ans, cerr := s.proClient().Check(ctx, ask)
		if cerr != nil {
			s.proMu.Unlock()
			return s.ProStatus(), cerr
		}
		if serr := s.Secrets.Set(proVault, proAccount, []byte(key)); serr != nil {
			s.proMu.Unlock()
			return s.ProStatus(), serr
		}
		err = s.applyPro(ctx, ans)
		s.proMu.Unlock()
		return s.ProStatus(), err
	}
	s.proMu.Unlock()
	if err != nil {
		return s.ProStatus(), err
	}
	// The certificate is stored; the normal check downloads the pack and
	// refreshes the cache. A pack download failure does not discard the key.
	_, err = s.ProCheck(ctx, true)
	return s.ProStatus(), err
}

// checkCertLocked refreshes the snapshot when due, then asks for the pack
// with a device-signed request.
func (s *Service) checkCertLocked(ctx context.Context) (pro.Answer, error) {
	var out pro.Answer
	cert, err := s.verifiedCert()
	if err != nil {
		return out, err
	}
	if cert.ExpiresAt <= s.now().Unix() || (cert.RefreshAfter > 0 && cert.RefreshAfter <= s.now().Unix()) {
		if fresh, rerr := s.refreshCertLocked(ctx, cert); rerr == nil {
			cert = fresh
		} else if errors.Is(rerr, pro.ErrUnsupported) {
			// A server too old to refresh cannot serve the new pack path.
			return out, rerr
		} else if s.Log != nil {
			s.Log.Warn("pro: refresh certificate", "err", rerr)
		}
	}
	seed, err := s.deviceSeed()
	if err != nil {
		return out, err
	}
	pub, err := pro.DevicePublicKey(seed)
	if err != nil {
		return out, err
	}
	at := s.now().Unix()
	sig, err := pro.DeviceProof(seed, cert.LicenseID, pub, at)
	if err != nil {
		return out, err
	}
	return s.proClient().CheckCert(ctx, pro.CertAsk{
		LicenseID: cert.LicenseID, DevicePubkey: pub, TS: at, Sig: sig,
		Have: pro.Installed(s.Dirs.Pro()), App: update.Current,
		Platform: runtime.GOOS + "-" + runtime.GOARCH,
	})
}

func (s *Service) refreshCertLocked(ctx context.Context, cert *pro.Certificate) (*pro.Certificate, error) {
	seed, err := s.deviceSeed()
	if err != nil {
		return nil, err
	}
	pub, err := pro.DevicePublicKey(seed)
	if err != nil {
		return nil, err
	}
	at := s.now().Unix()
	sig, err := pro.DeviceProof(seed, cert.LicenseID, pub, at)
	if err != nil {
		return nil, err
	}
	ans, err := s.proClient().Refresh(ctx, pro.RefreshRequest{
		LicenseID: cert.LicenseID, DevicePubkey: pub, TS: at, Sig: sig,
	})
	if err != nil {
		return nil, err
	}
	return s.acceptCert(ans.Cert, pub)
}

// ProRedeemGift exchanges a promo/paid gift code for a certificate. It does
// not need Stripe, and it can extend the licence already on this computer.
func (s *Service) ProRedeemGift(ctx context.Context, code string) (ProStatus, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return s.ProStatus(), apperr.New("pro_bad_key", "enter a gift code")
	}
	s.proMu.Lock()
	id, err := s.proInstallID()
	if err != nil {
		s.proMu.Unlock()
		return s.ProStatus(), err
	}
	seed, err := s.deviceSeed()
	if err != nil {
		s.proMu.Unlock()
		return s.ProStatus(), err
	}
	pub, err := pro.DevicePublicKey(seed)
	if err != nil {
		s.proMu.Unlock()
		return s.ProStatus(), err
	}
	current := ""
	if cert, cerr := s.verifiedCert(); cerr == nil {
		current = cert.LicenseID
	}
	ans, err := s.proClient().RedeemGift(ctx, pro.GiftRequest{
		Code: code, InstallID: id, DevicePubkey: pub, App: update.Current,
		Platform: runtime.GOOS + "-" + runtime.GOARCH, CurrentLicenseID: current,
	})
	if err != nil {
		s.proMu.Unlock()
		return s.ProStatus(), err
	}
	if ans.Key != "" {
		if err := s.Secrets.Set(proVault, proAccount, []byte(ans.Key)); err != nil {
			s.proMu.Unlock()
			return s.ProStatus(), err
		}
	}
	if _, err := s.acceptCert(ans.Cert, pub); err != nil {
		s.proMu.Unlock()
		return s.ProStatus(), err
	}
	s.proMu.Unlock()
	if _, err := s.ProCheck(ctx, true); err != nil {
		return s.ProStatus(), err
	}
	return s.ProStatus(), nil
}

// ProCheck asks the server about the key, at most once a day unless forced,
// and installs a newer pack it offers. Without a key it does nothing.
func (s *Service) ProCheck(ctx context.Context, force bool) (ProStatus, error) {
	key := s.proKey()
	if key == "" && s.proCertToken() == "" {
		return s.ProStatus(), nil
	}
	if !force {
		if t, err := time.Parse(time.RFC3339, s.proSaved().CheckedAt); err == nil && s.now().Sub(t) < proEvery {
			// Legacy customers (no certificate yet) keep the old once-a-day
			// behaviour; v2 customers skip only while the snapshot is fresh.
			if s.proCertToken() == "" {
				return s.ProStatus(), nil
			}
			if cert, cerr := s.verifiedCert(); cerr == nil && cert.RefreshAfter > s.now().Unix() {
				return s.ProStatus(), nil
			}
		}
	}
	s.proMu.Lock()
	defer s.proMu.Unlock()

	// First run after an upgrade: make the certificate once. An old server
	// answers 404, and the legacy key path below keeps that customer working.
	if s.proCertToken() == "" {
		if _, err := s.activateCertLocked(ctx, key); err != nil && !errors.Is(err, pro.ErrUnsupported) {
			return s.proCheckError(err)
		}
	}

	var ans pro.Answer
	var err error
	if s.proCertToken() != "" {
		ans, err = s.checkCertLocked(ctx)
	} else {
		var ask pro.Ask
		ask, err = s.proAsk(key)
		if err == nil {
			ans, err = s.proClient().Check(ctx, ask)
		}
	}
	if err != nil {
		return s.proCheckError(err)
	}
	err = s.applyPro(ctx, ans)
	return s.ProStatus(), err
}

func (s *Service) proCheckError(err error) (ProStatus, error) {
	p := s.proSaved()
	p.CheckedAt, p.Error = s.now().UTC().Format(time.RFC3339), err.Error()
	var ae *apperr.Error
	if errors.As(err, &ae) && (ae.Code == "pro_revoked" || ae.Code == "pro_bad_key") {
		// Refunded or disputed: the pack goes with the money.
		p.State, p.Runtimes = "revoked", nil
		s.clearCert()
		_ = pro.Remove(s.Dirs.Pro())
	}
	_ = s.saveProState(p)
	s.OfferMobileRuntime()
	s.notify("pro")
	return s.ProStatus(), err
}

// applyPro records the server's answer and installs the pack it offers.
func (s *Service) applyPro(ctx context.Context, ans pro.Answer) error {
	p := s.proSaved()
	p.State, p.PaidThrough, p.CheckedAt, p.Error = ans.State, ans.PaidThrough, s.now().UTC().Format(time.RFC3339), ""
	// An active lifetime licence may have passed its updates window, so the
	// server stops offering new runtimes. Keep the runtime already installed
	// instead of clearing it and making existing Mobile devices unopenable.
	if ans.State != "active" || len(ans.Runtimes) > 0 {
		p.Runtimes = ans.Runtimes
	}
	defer s.notify("pro")
	defer s.OfferMobileRuntime()
	if ans.Release == nil {
		return s.saveProState(p)
	}
	dir := filepath.Join(s.Dirs.Pro(), "downloads")
	zp, err := s.proClient().Download(ctx, *ans.Release, dir)
	if err == nil {
		err = pro.Install(zp, ans.Release.Version, s.Dirs.Pro())
		os.RemoveAll(dir)
	}
	if err != nil {
		p.Error = err.Error()
		_ = s.saveProState(p)
		return err
	}
	p.Notes = ans.Release.Notes
	if s.Log != nil {
		s.Log.Info("pro pack installed", "version", ans.Release.Version)
	}
	return s.saveProState(p)
}

// ProDeactivate removes the key and the pack from this computer and frees
// its place on the key (when the server can be reached; else it frees itself
// after 30 days).
func (s *Service) ProDeactivate(ctx context.Context) error {
	s.proMu.Lock()
	defer s.proMu.Unlock()
	if key := s.proKey(); key != "" {
		if id, _ := s.Store.GetSetting("pro_install_id"); id != "" {
			if err := s.proClient().RemoveDevice(ctx, key, id); err != nil && s.Log != nil {
				s.Log.Warn("pro: free this computer's place", "err", err)
			}
		}
	}
	if err := s.Secrets.Delete(proVault, proAccount); err != nil {
		return err
	}
	s.clearCert()
	_ = s.Store.SetSetting("pro_state", "")
	s.OfferMobileRuntime()
	defer s.notify("pro")
	return pro.Remove(s.Dirs.Pro())
}

// ProPortalURL is the Stripe page where the user manages the subscription.
func (s *Service) ProPortalURL(ctx context.Context) (string, error) {
	key := s.proKey()
	if key == "" {
		return "", apperr.New("pro_bad_key", "no key on this computer")
	}
	return s.proClient().Portal(ctx, key)
}

// ProBuyURL starts a purchase: the page to open, and the ref to claim its key with.
func (s *Service) ProBuyURL(plan string) (url, ref string, err error) {
	if plan != "month" && plan != "year" {
		return "", "", apperr.New("pro_bad_plan", "plan is month or year")
	}
	ref = pro.NewRef()
	return s.proClient().BuyURL(plan, s.Language(), ref), ref, nil
}

// ProBuyGiftURL is the page that sells a paid gift code. The code is emailed
// after payment; it is not a subscription and does not activate this computer.
func (s *Service) ProBuyGiftURL() string {
	return s.proClient().BuyURL("gift", s.Language(), "")
}

// ProClaim takes the key of a paid purchase and activates it; false while
// the purchase is not paid yet.
func (s *Service) ProClaim(ctx context.Context, ref string) (bool, error) {
	key, err := s.proClient().Claim(ctx, ref)
	if errors.Is(err, pro.ErrNotYet) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = s.ProActivate(ctx, key)
	return true, err
}

// ProScripts is the installed pack's scripts folder, "" without a pack.
func (s *Service) ProScripts() string {
	if pro.Installed(s.Dirs.Pro()) == "" {
		return ""
	}
	return pro.Scripts(s.Dirs.Pro())
}

// ProDevices lists this licence's machines; the request is signed by the
// current device key, so the result is only available on an activated computer.
func (s *Service) ProDevices() ([]pro.DeviceInfo, error) {
	cert, err := s.verifiedCert()
	if err != nil {
		return nil, err
	}
	seed, err := s.deviceSeed()
	if err != nil {
		return nil, err
	}
	pub, err := pro.DevicePublicKey(seed)
	if err != nil {
		return nil, err
	}
	at := s.now().Unix()
	sig, err := pro.DeviceProof(seed, cert.LicenseID, pub, at)
	if err != nil {
		return nil, err
	}
	return s.proClient().Devices(context.Background(), pro.DeviceProofRequest{
		LicenseID: cert.LicenseID, DevicePubkey: pub, TS: at, Sig: sig,
	})
}

// ProRemoveDevice frees another machine's slot. Removing the current machine
// makes its next refresh fail; the app asks before doing that.
func (s *Service) ProRemoveDevice(installID string) error {
	cert, err := s.verifiedCert()
	if err != nil {
		return err
	}
	seed, err := s.deviceSeed()
	if err != nil {
		return err
	}
	pub, err := pro.DevicePublicKey(seed)
	if err != nil {
		return err
	}
	at := s.now().Unix()
	sig, err := pro.DeviceProof(seed, cert.LicenseID, pub, at)
	if err != nil {
		return err
	}
	return s.proClient().RemoveDeviceV2(context.Background(), pro.DeviceProofRequest{
		LicenseID: cert.LicenseID, DevicePubkey: pub, TS: at, Sig: sig,
	}, installID)
}
