// SPDX-License-Identifier: AGPL-3.0-or-later

package pro

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

// Account tokens are short-lived access plus rotating refresh tokens. They
// live in the desktop vault, never in SQLite.
type AccountTokens struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	AccessExpiresAt  int64  `json:"access_expires_at"`
	RefreshExpiresAt int64  `json:"refresh_expires_at"`
}

type AccountLicense struct {
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

type Account struct {
	SignedIn bool            `json:"signed_in"`
	Email    string          `json:"email"`
	Name     string          `json:"name"`
	License  *AccountLicense `json:"license"`
}

type AccountDevice struct {
	InstallID string `json:"install_id"`
	Name      string `json:"name"`
	Platform  string `json:"platform"`
	LastSeen  int64  `json:"last_seen"`
	Current   bool   `json:"current"`
}

func (c Client) doAccount(ctx context.Context, method, path, token string, in, out any) (int, error) {
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
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return 0, apperr.Newf("pro_offline", "cannot reach the AnyProfile Pro server: %v", err)
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

// ExchangeTicket trades the browser callback ticket for account tokens.
func (c Client) ExchangeTicket(ctx context.Context, ticket, verifier, installID string) (AccountTokens, Account, error) {
	var out struct {
		Tokens  AccountTokens `json:"tokens"`
		Account Account       `json:"account"`
	}
	_, err := c.doAccount(ctx, http.MethodPost, "/v1/auth/device", "", map[string]string{
		"ticket": ticket, "verifier": verifier, "install_id": installID,
	}, &out)
	return out.Tokens, out.Account, err
}

func (c Client) RefreshAccount(ctx context.Context, refresh, installID string) (AccountTokens, Account, error) {
	var out struct {
		Tokens  AccountTokens `json:"tokens"`
		Account Account       `json:"account"`
	}
	_, err := c.doAccount(ctx, http.MethodPost, "/v1/auth/refresh", "", map[string]string{
		"refresh_token": refresh, "install_id": installID,
	}, &out)
	return out.Tokens, out.Account, err
}

func (c Client) Account(ctx context.Context, access string) (Account, error) {
	var out Account
	_, err := c.doAccount(ctx, http.MethodGet, "/v1/account", access, nil, &out)
	return out, err
}

func (c Client) AccountLogout(ctx context.Context, access string) error {
	_, err := c.doAccount(ctx, http.MethodPost, "/v1/auth/logout", access, nil, nil)
	return err
}

func (c Client) AccountActivate(ctx context.Context, access string, in ActivateRequest) (ActivateAnswer, error) {
	var out ActivateAnswer
	_, err := c.doAccount(ctx, http.MethodPost, "/v1/account/activate", access, in, &out)
	return out, err
}

func (c Client) AccountLink(ctx context.Context, access, key string) (Account, error) {
	var out Account
	_, err := c.doAccount(ctx, http.MethodPost, "/v1/account/link-license", access, map[string]string{"key": key}, &out)
	return out, err
}

func (c Client) AccountDevices(ctx context.Context, access string) ([]AccountDevice, error) {
	var out []AccountDevice
	_, err := c.doAccount(ctx, http.MethodGet, "/v1/account/devices", access, nil, &out)
	return out, err
}

func (c Client) AccountRemoveDevice(ctx context.Context, access, installID string) error {
	_, err := c.doAccount(ctx, http.MethodPost, "/v1/account/devices/remove", access, map[string]string{"install_id": installID}, nil)
	return err
}
