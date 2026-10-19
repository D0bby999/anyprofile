// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/pro"
	"github.com/D0bby999/anyprofile/internal/update"
)

// The Pro key lives in the vault with the other secrets.
const (
	proVault   = "anyprofile-pro"
	proAccount = "license"
	proEvery   = 24 * time.Hour
)

// ProStatus is what Settings → Pro shows.
type ProStatus struct {
	Active      bool   `json:"active"`      // a key is entered on this computer
	Key         string `json:"key"`         // the key with all but its last group hidden
	State       string `json:"state"`       // pending | active | past_due | expired | revoked; "" before the first check
	PaidThrough string `json:"paidThrough"` // RFC 3339, "" when unknown
	Version     string `json:"version"`     // the installed pack, "" when none
	Notes       string `json:"notes"`       // what the installed pack changed
	CheckedAt   string `json:"checkedAt"`   // RFC 3339, "" when never
	Error       string `json:"error"`       // the last check's failure, "" when it went well
	Waiting     bool   `json:"waiting"`     // a purchase started here waits for payment (set by the app)
}

type proSaved struct {
	State       string `json:"state"`
	PaidThrough int64  `json:"paidThrough"`
	CheckedAt   string `json:"checkedAt"`
	Notes       string `json:"notes"`
	Error       string `json:"error"`
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
	p := s.proSaved()
	st := ProStatus{Active: key != "", State: p.State, CheckedAt: p.CheckedAt, Error: p.Error,
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
	return st
}

func (s *Service) proAsk(key string) (pro.Ask, error) {
	id, err := s.proInstallID()
	return pro.Ask{Key: key, InstallID: id, Have: pro.Installed(s.Dirs.Pro()), App: update.Current,
		Platform: runtime.GOOS + "-" + runtime.GOARCH}, err
}

// ProActivate checks a key with the server and, when it is good, keeps it
// and installs the pack it may have. A key the server refuses is not kept.
func (s *Service) ProActivate(ctx context.Context, key string) (ProStatus, error) {
	key = strings.ToUpper(strings.TrimSpace(key))
	if key == "" {
		return s.ProStatus(), apperr.New("pro_bad_key", "enter a key")
	}
	s.proMu.Lock()
	defer s.proMu.Unlock()
	ask, err := s.proAsk(key)
	if err != nil {
		return s.ProStatus(), err
	}
	ans, err := s.proClient().Check(ctx, ask)
	if err != nil {
		return s.ProStatus(), err
	}
	if err := s.Secrets.Set(proVault, proAccount, []byte(key)); err != nil {
		return s.ProStatus(), err
	}
	err = s.applyPro(ctx, ans)
	return s.ProStatus(), err
}

// ProCheck asks the server about the key, at most once a day unless forced,
// and installs a newer pack it offers. Without a key it does nothing.
func (s *Service) ProCheck(ctx context.Context, force bool) (ProStatus, error) {
	key := s.proKey()
	if key == "" {
		return s.ProStatus(), nil
	}
	if !force {
		if t, err := time.Parse(time.RFC3339, s.proSaved().CheckedAt); err == nil && s.now().Sub(t) < proEvery {
			return s.ProStatus(), nil
		}
	}
	s.proMu.Lock()
	defer s.proMu.Unlock()
	ask, err := s.proAsk(key)
	if err != nil {
		return s.ProStatus(), err
	}
	ans, err := s.proClient().Check(ctx, ask)
	if err != nil {
		p := s.proSaved()
		p.CheckedAt, p.Error = s.now().UTC().Format(time.RFC3339), err.Error()
		var ae *apperr.Error
		if errors.As(err, &ae) && (ae.Code == "pro_revoked" || ae.Code == "pro_bad_key") {
			// Refunded or disputed: the pack goes with the money.
			p.State = "revoked"
			_ = pro.Remove(s.Dirs.Pro())
		}
		_ = s.saveProState(p)
		s.notify("pro")
		return s.ProStatus(), err
	}
	err = s.applyPro(ctx, ans)
	return s.ProStatus(), err
}

// applyPro records the server's answer and installs the pack it offers.
func (s *Service) applyPro(ctx context.Context, ans pro.Answer) error {
	p := s.proSaved()
	p.State, p.PaidThrough, p.CheckedAt, p.Error = ans.State, ans.PaidThrough, s.now().UTC().Format(time.RFC3339), ""
	defer s.notify("pro")
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
	_ = s.Store.SetSetting("pro_state", "")
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
