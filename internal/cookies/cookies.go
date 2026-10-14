// SPDX-License-Identifier: AGPL-3.0-or-later

// Package cookies reads and writes cookie files: the JSON of Cookie-Editor and
// EditThisCookie, and Netscape cookies.txt. Cookies are shaped like the
// DevTools Network.CookieParam, ready for Storage.setCookies.
package cookies

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// MaxCookies is the most a file may hold.
	MaxCookies = 5000
	// MaxFileBytes is checked by the caller before reading.
	MaxFileBytes = 5 << 20
	maxValue     = 4096
)

// Cookie is one cookie in DevTools form. Expires is seconds since the epoch;
// zero means a session cookie.
type Cookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Secure   bool    `json:"secure"`
	HTTPOnly bool    `json:"httpOnly"`
	SameSite string  `json:"sameSite,omitempty"` // Strict, Lax, None or empty
	Expires  float64 `json:"expires,omitempty"`
}

var (
	ErrTooMany = fmt.Errorf("more than %d cookies", MaxCookies)
	ErrEmpty   = errors.New("no cookies found")
)

// Parse reads a cookie file and returns the usable cookies and how many were skipped.
func Parse(data []byte) ([]Cookie, int, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	trimmed := bytes.TrimSpace(data)
	var all []Cookie
	var err error
	if len(trimmed) > 0 && (trimmed[0] == '[' || trimmed[0] == '{') {
		all, err = parseJSON(trimmed)
	} else {
		all, err = parseNetscape(data)
	}
	if err != nil {
		return nil, 0, err
	}
	if len(all) > MaxCookies {
		return nil, 0, ErrTooMany
	}
	now := float64(time.Now().Unix())
	var out []Cookie
	skipped := 0
	for _, c := range all {
		if c.Name == "" || c.Domain == "" || len(c.Value) > maxValue || (c.Expires > 0 && c.Expires < now) {
			skipped++
			continue
		}
		if c.Path == "" {
			c.Path = "/"
		}
		out = append(out, c)
	}
	if len(out) == 0 && skipped == 0 {
		return nil, 0, ErrEmpty
	}
	return out, skipped, nil
}

// jsonCookie accepts the field names both browser extensions use.
type jsonCookie struct {
	Name           string   `json:"name"`
	Value          string   `json:"value"`
	Domain         string   `json:"domain"`
	Path           string   `json:"path"`
	Secure         bool     `json:"secure"`
	HTTPOnly       bool     `json:"httpOnly"`
	SameSite       string   `json:"sameSite"`
	ExpirationDate *float64 `json:"expirationDate"`
	Expires        *float64 `json:"expires"`
	Session        bool     `json:"session"`
}

func parseJSON(b []byte) ([]Cookie, error) {
	var list []jsonCookie
	if b[0] == '{' {
		var wrap struct {
			Cookies []jsonCookie `json:"cookies"`
		}
		if err := json.Unmarshal(b, &wrap); err != nil {
			return nil, jsonError(b, err)
		}
		list = wrap.Cookies
	} else if err := json.Unmarshal(b, &list); err != nil {
		return nil, jsonError(b, err)
	}
	out := make([]Cookie, 0, len(list))
	for _, j := range list {
		c := Cookie{Name: j.Name, Value: j.Value, Domain: j.Domain, Path: j.Path, Secure: j.Secure,
			HTTPOnly: j.HTTPOnly, SameSite: sameSite(j.SameSite)}
		switch {
		case j.Session:
		case j.ExpirationDate != nil:
			c.Expires = *j.ExpirationDate
		case j.Expires != nil && *j.Expires > 0:
			c.Expires = *j.Expires
		}
		out = append(out, c)
	}
	return out, nil
}

func jsonError(b []byte, err error) error {
	var se *json.SyntaxError
	if errors.As(err, &se) {
		line := bytes.Count(b[:min(int(se.Offset), len(b))], []byte("\n")) + 1
		return fmt.Errorf("not valid JSON at line %d", line)
	}
	return fmt.Errorf("not a cookie list: %v", err)
}

func sameSite(v string) string {
	switch strings.ToLower(v) {
	case "no_restriction", "none":
		return "None"
	case "lax":
		return "Lax"
	case "strict":
		return "Strict"
	}
	return ""
}

func parseNetscape(data []byte) ([]Cookie, error) {
	var out []Cookie
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64<<10), MaxFileBytes)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		httpOnly := false
		if strings.HasPrefix(line, "#HttpOnly_") {
			line, httpOnly = strings.TrimPrefix(line, "#HttpOnly_"), true
		} else if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 7 {
			continue
		}
		exp, err := strconv.ParseFloat(f[4], 64)
		if err != nil || math.IsNaN(exp) {
			continue
		}
		out = append(out, Cookie{Domain: f[0], Path: f[2], Secure: strings.EqualFold(f[3], "TRUE"),
			Expires: exp, Name: f[5], Value: f[6], HTTPOnly: httpOnly})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New("no valid cookies.txt line found")
	}
	return out, nil
}

// exported is the Cookie-Editor JSON shape.
type exported struct {
	Domain         string   `json:"domain"`
	Name           string   `json:"name"`
	Value          string   `json:"value"`
	Path           string   `json:"path"`
	Secure         bool     `json:"secure"`
	HTTPOnly       bool     `json:"httpOnly"`
	SameSite       string   `json:"sameSite"`
	ExpirationDate *float64 `json:"expirationDate,omitempty"`
	HostOnly       bool     `json:"hostOnly"`
	Session        bool     `json:"session"`
}

// Export writes cookies as a Cookie-Editor JSON array.
func Export(cs []Cookie) ([]byte, error) {
	out := make([]exported, 0, len(cs))
	for _, c := range cs {
		e := exported{Domain: c.Domain, Name: c.Name, Value: c.Value, Path: c.Path, Secure: c.Secure,
			HTTPOnly: c.HTTPOnly, HostOnly: !strings.HasPrefix(c.Domain, "."), Session: c.Expires <= 0}
		switch c.SameSite {
		case "None":
			e.SameSite = "no_restriction"
		case "":
			e.SameSite = "unspecified"
		default:
			e.SameSite = strings.ToLower(c.SameSite)
		}
		if c.Expires > 0 {
			exp := c.Expires
			e.ExpirationDate = &exp
		}
		out = append(out, e)
	}
	return json.MarshalIndent(out, "", "  ")
}

// TopDomains is the registrable part of each cookie domain (no leading dot or
// "www."), most cookies first.
func TopDomains(cs []Cookie, n int) []string {
	count := map[string]int{}
	for _, c := range cs {
		d := strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(c.Domain), "."), "www.")
		count[d]++
	}
	out := make([]string, 0, len(count))
	for d := range count {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		return count[out[i]] > count[out[j]] || count[out[i]] == count[out[j]] && out[i] < out[j]
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}
