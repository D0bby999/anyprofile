// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/cookies"
)

// cookieBatch is how many cookies go in one Storage.setCookies call.
const cookieBatch = 200

// CookieImport reports an import. Cookie values never leave the backend.
type CookieImport struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Domains  []string `json:"domains"`
	// SiteMismatch is the main cookie domain when it does not belong to the
	// profile's platform; empty when it does or the platform is Other.
	SiteMismatch string `json:"siteMismatch"`
}

// readCookieFile reads and parses a cookie file, refusing big or broken ones.
func readCookieFile(path string) ([]cookies.Cookie, int, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, 0, err
	}
	if st.Size() > cookies.MaxFileBytes {
		return nil, 0, apperr.New("cookies_file_too_big", "the file is larger than 5 MB")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	cs, skipped, err := cookies.Parse(b)
	switch {
	case errors.Is(err, cookies.ErrTooMany):
		return nil, 0, apperr.New("cookies_too_many", "more than 5000 cookies")
	case err != nil:
		return nil, 0, apperr.Newf("cookies_bad_format", "%v", err)
	}
	return cs, skipped, nil
}

// siteMismatch returns top when it is not one of the platform's domains (or a subdomain of one).
func siteMismatch(top string, siteDomains []string) string {
	if top == "" || len(siteDomains) == 0 {
		return ""
	}
	for _, d := range siteDomains {
		if top == d || strings.HasSuffix(top, "."+d) || strings.HasSuffix(d, "."+top) {
			return ""
		}
	}
	return top
}

// siteDomains joins the domains of every platform the profile is on.
func (s *Service) siteDomains(siteIDs []string) []string {
	all, err := s.Sites()
	if err != nil {
		return nil
	}
	var out []string
	for _, x := range all {
		if slices.Contains(siteIDs, x.ID) {
			out = append(out, x.Domains...)
		}
	}
	return out
}

// ImportCookies puts a cookie file into a profile. A closed profile is saved
// as a snapshot first, then opened to receive the cookies.
func (s *Service) ImportCookies(ctx context.Context, profileID, path string) (CookieImport, error) {
	cs, skipped, err := readCookieFile(path)
	if err != nil {
		return CookieImport{}, err
	}
	p, err := s.Store.GetProfile(profileID)
	if err != nil {
		return CookieImport{}, err
	}
	conn, ok := s.Launcher.Conn(profileID)
	if !ok && s.running()[profileID] {
		return CookieImport{}, apperr.New("cookies_need_restart", "close and reopen the profile, then import again")
	}
	if !ok {
		if _, err := s.TakeSnapshot(profileID, "before-import"); err != nil && !strings.Contains(err.Error(), "nothing_to_snapshot") {
			return CookieImport{}, err
		}
		if err := s.Launcher.Start(ctx, profileID); err != nil {
			return CookieImport{}, err
		}
		wctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		conn, err = s.Launcher.WaitConn(wctx, profileID)
		cancel()
		if err != nil {
			return CookieImport{}, err
		}
	}
	for i := 0; i < len(cs); i += cookieBatch {
		batch := cs[i:min(i+cookieBatch, len(cs))]
		if _, err := conn.Call(ctx, "", "Storage.setCookies", map[string]any{"cookies": batch}); err != nil {
			return CookieImport{}, err
		}
	}
	top := cookies.TopDomains(cs, 5)
	res := CookieImport{Imported: len(cs), Skipped: skipped, Domains: top}
	if len(top) > 0 {
		res.SiteMismatch = siteMismatch(top[0], s.siteDomains(p.Sites))
	}
	s.Log.Info("cookies imported", "profile", profileID, "count", len(cs), "skipped", skipped, "domains", strings.Join(top, ","))
	return res, nil
}

// ExportCookies writes the cookies of a running profile to path, owner-only.
func (s *Service) ExportCookies(ctx context.Context, profileID, path string) (int, error) {
	conn, ok := s.Launcher.Conn(profileID)
	if !ok {
		return 0, apperr.New("cookies_need_running", "open the profile first")
	}
	raw, err := conn.Call(ctx, "", "Storage.getCookies", map[string]any{})
	if err != nil {
		return 0, err
	}
	var got struct {
		Cookies []cookies.Cookie `json:"cookies"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		return 0, err
	}
	for i := range got.Cookies {
		if got.Cookies[i].Expires < 0 { // DevTools gives -1 for session cookies
			got.Cookies[i].Expires = 0
		}
	}
	b, err := cookies.Export(got.Cookies)
	if err != nil {
		return 0, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cookies-*")
	if err != nil {
		return 0, err
	}
	if _, err = tmp.Write(b); err == nil {
		err = tmp.Chmod(0o600)
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		os.Remove(tmp.Name())
		return 0, err
	}
	s.Log.Info("cookies exported", "profile", profileID, "count", len(got.Cookies))
	return len(got.Cookies), nil
}
