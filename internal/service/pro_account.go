// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/pro"
	"github.com/D0bby999/anyprofile/internal/update"
)

const (
	accountAccess  = "account-access"
	accountRefresh = "account-refresh"
	accountEmail   = "account-email"
)

// ProAccountStatus is what Settings → Pro shows after a sign-in.
type ProAccountStatus struct {
	SignedIn bool                `json:"signedIn"`
	Email    string              `json:"email"`
	Name     string              `json:"name"`
	License  *pro.AccountLicense `json:"license,omitempty"`
	Error    string              `json:"error,omitempty"`
}

type proAccountLogin struct {
	server   *http.Server
	listener net.Listener
	state    string
	verifier string
	result   chan string
	err      chan error
}

func randomURLToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *Service) accountTokens() (pro.AccountTokens, bool) {
	var t pro.AccountTokens
	b, err := s.Secrets.Get(proVault, accountAccess)
	if err != nil {
		return t, false
	}
	if err := json.Unmarshal(b, &t); err != nil || t.AccessToken == "" || t.RefreshToken == "" {
		return t, false
	}
	return t, true
}

func (s *Service) saveAccountTokens(t pro.AccountTokens) error {
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return s.Secrets.Set(proVault, accountAccess, b)
}

func (s *Service) clearAccountTokens() {
	_ = s.Secrets.Delete(proVault, accountAccess)
	_ = s.Secrets.Delete(proVault, accountEmail)
}

func (s *Service) saveAccountEmail(email string) {
	_ = s.Secrets.Set(proVault, accountEmail, []byte(email))
}

// ProAccountStatus reads the account, refreshing once when the access token
// has expired. Offline or signed-out returns SignedIn=false, not an error.
func (s *Service) ProAccountStatus(ctx context.Context) (ProAccountStatus, error) {
	t, ok := s.accountTokens()
	if !ok {
		return ProAccountStatus{SignedIn: false}, nil
	}
	client := s.proClient()
	a, err := client.Account(ctx, t.AccessToken)
	if err != nil {
		refreshed, ra, rerr := client.RefreshAccount(ctx, t.RefreshToken, s.mustInstallID())
		if rerr != nil {
			return ProAccountStatus{SignedIn: false, Error: rerr.Error()}, nil
		}
		t = refreshed
		if err := s.saveAccountTokens(t); err != nil {
			return ProAccountStatus{SignedIn: false, Error: err.Error()}, nil
		}
		a = ra
	}
	if a.Email != "" {
		s.saveAccountEmail(a.Email)
	}
	return ProAccountStatus{SignedIn: true, Email: a.Email, Name: a.Name, License: a.License}, nil
}

func (s *Service) mustInstallID() string {
	id, err := s.proInstallID()
	if err != nil {
		return ""
	}
	return id
}

// ProAccountLoginStart opens a loopback listener and returns the browser URL.
// The caller opens it, then calls ProAccountLoginWait.
func (s *Service) ProAccountLoginStart() (string, error) {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	if s.accountLogin != nil {
		s.closeAccountLoginLocked()
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	state, err := randomURLToken(32)
	if err != nil {
		ln.Close()
		return "", err
	}
	verifier, err := randomURLToken(48)
	if err != nil {
		ln.Close()
		return "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	l := &proAccountLogin{listener: ln, state: state, verifier: verifier, result: make(chan string, 1), err: make(chan error, 1)}
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		ticket := r.URL.Query().Get("ticket")
		if ticket == "" {
			http.Error(w, "missing ticket", http.StatusBadRequest)
			return
		}
		select {
		case l.result <- ticket:
		default:
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><meta charset=utf-8><title>AnyProfile</title><body style=\"font-family:system-ui;padding:40px\">Signed in. You can close this tab and return to AnyProfile.<script>history.replaceState(null, \"\", \"/callback\");setTimeout(function(){window.close()},600)</script></body>"))
	})
	l.server = &http.Server{Handler: mux}
	go func() {
		if err := l.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			select {
			case l.err <- err:
			default:
			}
		}
	}()
	s.accountLogin = l
	q := url.Values{"port": {fmt.Sprint(port)}, "state": {state}, "challenge": {challenge}}
	return strings.TrimRight(s.proClient().Base, "/") + "/auth/google/start?" + q.Encode(), nil
}

func (s *Service) closeAccountLoginLocked() {
	if s.accountLogin == nil {
		return
	}
	_ = s.accountLogin.server.Close()
	_ = s.accountLogin.listener.Close()
	s.accountLogin = nil
}

// ProAccountLoginWait waits for the browser callback, exchanges the ticket,
// stores the session in the vault, and activates this device.
func (s *Service) ProAccountLoginWait(ctx context.Context) (ProAccountStatus, error) {
	s.accountMu.Lock()
	l := s.accountLogin
	s.accountMu.Unlock()
	if l == nil {
		return ProAccountStatus{}, apperr.New("account_login", "sign-in was not started")
	}
	timer := time.NewTimer(5 * time.Minute)
	defer timer.Stop()
	var ticket string
	select {
	case ticket = <-l.result:
	case err := <-l.err:
		s.accountMu.Lock()
		s.closeAccountLoginLocked()
		s.accountMu.Unlock()
		return ProAccountStatus{}, err
	case <-ctx.Done():
		s.accountMu.Lock()
		s.closeAccountLoginLocked()
		s.accountMu.Unlock()
		return ProAccountStatus{}, ctx.Err()
	case <-timer.C:
		s.accountMu.Lock()
		s.closeAccountLoginLocked()
		s.accountMu.Unlock()
		return ProAccountStatus{}, apperr.New("account_timeout", "sign-in timed out")
	}
	s.accountMu.Lock()
	s.closeAccountLoginLocked()
	s.accountMu.Unlock()
	install := s.mustInstallID()
	tokens, a, err := s.proClient().ExchangeTicket(ctx, ticket, l.verifier, install)
	if err != nil {
		return ProAccountStatus{}, err
	}
	if err := s.saveAccountTokens(tokens); err != nil {
		return ProAccountStatus{}, err
	}
	if a.Email != "" {
		s.saveAccountEmail(a.Email)
	}
	if err := s.activateAccount(ctx, tokens.AccessToken, a); err != nil {
		return ProAccountStatus{SignedIn: true, Email: a.Email}, err
	}
	return s.ProAccountStatus(ctx)
}

func (s *Service) activateAccount(ctx context.Context, access string, a pro.Account) error {
	seed, err := s.deviceSeed()
	if err != nil {
		return err
	}
	pub, err := pro.DevicePublicKey(seed)
	if err != nil {
		return err
	}
	install := s.mustInstallID()
	ans, err := s.proClient().AccountActivate(ctx, access, pro.ActivateRequest{
		InstallID: install, DevicePubkey: pub, App: update.Current, Platform: runtime.GOOS + "-" + runtime.GOARCH,
	})
	if err != nil {
		return err
	}
	_, err = s.acceptCert(ans.Cert, pub)
	return err
}

func (s *Service) ProAccountLink(ctx context.Context, key string) (ProAccountStatus, error) {
	t, ok := s.accountTokens()
	if !ok {
		return ProAccountStatus{}, apperr.New("account_auth", "sign in first")
	}
	a, err := s.proClient().AccountLink(ctx, t.AccessToken, strings.TrimSpace(key))
	if err != nil {
		return ProAccountStatus{}, err
	}
	if err := s.activateAccount(ctx, t.AccessToken, a); err != nil {
		return ProAccountStatus{SignedIn: true, Email: a.Email}, err
	}
	return s.ProAccountStatus(ctx)
}

func (s *Service) ProAccountSignOut(ctx context.Context) error {
	t, ok := s.accountTokens()
	if !ok {
		return nil
	}
	_ = s.proClient().AccountLogout(ctx, t.AccessToken)
	s.clearAccountTokens()
	return nil
}

func (s *Service) ProAccountDevices(ctx context.Context) ([]pro.AccountDevice, error) {
	t, ok := s.accountTokens()
	if !ok {
		return nil, apperr.New("account_auth", "sign in first")
	}
	devs, err := s.proClient().AccountDevices(ctx, t.AccessToken)
	if err == nil {
		return devs, nil
	}
	t2, _, rerr := s.proClient().RefreshAccount(ctx, t.RefreshToken, s.mustInstallID())
	if rerr != nil {
		return nil, err
	}
	if err := s.saveAccountTokens(t2); err != nil {
		return nil, err
	}
	return s.proClient().AccountDevices(ctx, t2.AccessToken)
}

func (s *Service) ProAccountRemoveDevice(ctx context.Context, installID string) error {
	t, ok := s.accountTokens()
	if !ok {
		return apperr.New("account_auth", "sign in first")
	}
	return s.proClient().AccountRemoveDevice(ctx, t.AccessToken, installID)
}

// ProAccountActivate binds this computer to the signed-in account's active
// licence. Useful after a licence was linked to the account from another
// machine or by support.
func (s *Service) ProAccountActivate(ctx context.Context) (ProStatus, error) {
	t, ok := s.accountTokens()
	if !ok {
		return s.ProStatus(), apperr.New("account_auth", "sign in first")
	}
	a, err := s.proClient().Account(ctx, t.AccessToken)
	if err != nil {
		return s.ProStatus(), err
	}
	if a.License == nil {
		return s.ProStatus(), apperr.New("account_no_entitlement", "this account has no active Pro licence")
	}
	if err := s.activateAccount(ctx, t.AccessToken, a); err != nil {
		return s.ProStatus(), err
	}
	return s.ProStatus(), nil
}
