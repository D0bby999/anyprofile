// SPDX-License-Identifier: AGPL-3.0-or-later

package netproxy

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ParsedLine is one line of a pasted proxy list.
type ParsedLine struct {
	Line     int      `json:"line"`
	Upstream Upstream `json:"-"`
	Note     string   `json:"-"`
	Display  string   `json:"display"` // redacted
	Err      string   `json:"err"`
	// Code names the problem for the UI to put in words: not_url, scheme,
	// host_port, format or port.
	Code string `json:"code"`
}

// parseError is a line that is not a proxy, with a code the UI translates.
type parseError struct{ code, msg string }

func (e parseError) Error() string { return e.msg }

func errf(code, format string, a ...any) error { return parseError{code, fmt.Sprintf(format, a...)} }

// ParseList accepts one proxy per line, in either form:
//
//	scheme://user:pass@host:port   (scheme http, https, socks5, socks5h)
//	host:port[:user:pass]          (http assumed)
//
// Blank lines and lines starting with # are skipped.
func ParseList(text string) []ParsedLine {
	var out []ParsedLine
	for i, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		up, err := parseOne(line)
		var note string
		if err == nil {
			up, note = normalizeOxylabs(up)
		}
		p := ParsedLine{Line: i + 1, Upstream: up, Note: note}
		if err != nil {
			p.Err = err.Error()
			if pe, ok := err.(parseError); ok {
				p.Code = pe.code
			}
		} else {
			p.Display = up.Redacted()
		}
		out = append(out, p)
	}
	return out
}

// NormalizeScheme returns the canonical proxy scheme ("http", "https", "socks5").
func NormalizeScheme(s string) (string, bool) {
	canon, ok := map[string]string{"http": "http", "https": "https", "socks5": "socks5", "socks5h": "socks5"}[strings.ToLower(strings.TrimSpace(s))]
	return canon, ok
}

func parseOne(line string) (Upstream, error) {
	line = strings.TrimSpace(line)
	scheme := "http"

	if idx := strings.Index(line, "://"); idx != -1 {
		rawScheme := line[:idx]
		norm, ok := NormalizeScheme(rawScheme)
		if !ok {
			return Upstream{}, errf("scheme", "unsupported scheme %q", rawScheme)
		}
		scheme = norm
		rest := line[idx+3:]
		if strings.Contains(rest, "@") {
			u, err := url.Parse(scheme + "://" + rest)
			if err != nil {
				return Upstream{}, errf("not_url", "not a proxy URL")
			}
			port, err := strconv.Atoi(u.Port())
			if err != nil || u.Hostname() == "" {
				return Upstream{}, errf("host_port", "needs host:port")
			}
			pass, _ := u.User.Password()
			return check(Upstream{Scheme: scheme, Host: u.Hostname(), Port: port, Username: u.User.Username(), Password: pass})
		}
		line = rest
	} else if idx := strings.Index(line, ":"); idx != -1 {
		if norm, ok := NormalizeScheme(line[:idx]); ok {
			scheme = norm
			line = strings.TrimPrefix(line[idx+1:], "//")
		}
	}

	parts := strings.Split(line, ":")

	if len(parts) == 3 {
		if norm, ok := NormalizeScheme(parts[2]); ok {
			scheme = norm
			parts = parts[:2]
		}
	} else if len(parts) >= 5 {
		if norm, ok := NormalizeScheme(parts[len(parts)-1]); ok {
			scheme = norm
			parts = parts[:len(parts)-1]
		}
	}

	if len(parts) != 2 && len(parts) < 4 {
		return Upstream{}, errf("format", "expected host:port or host:port:user:pass")
	}
	if parts[0] == "" {
		return Upstream{}, errf("host_port", "needs host:port")
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return Upstream{}, errf("port", "bad port %q", parts[1])
	}
	up := Upstream{Scheme: scheme, Host: parts[0], Port: port}
	if len(parts) >= 4 {
		// A password may itself contain ':'.
		up.Username, up.Password = parts[2], strings.Join(parts[3:], ":")
	}
	return check(up)
}

func check(up Upstream) (Upstream, error) {
	if up.Host == "" || up.Port < 1 || up.Port > 65535 {
		return up, errf("host_port", "bad host or port")
	}
	return up, nil
}

// normalizeOxylabs fixes Oxylabs dashboard CSV exports, which write the
// assigned exit IP instead of the gateway (disp/isp/ddc.oxylabs.io) and omit
// the mandatory user-/customer- username prefix.
func normalizeOxylabs(up Upstream) (Upstream, string) {
	if up.Username == "" {
		return up, ""
	}
	raw := strings.TrimPrefix(up.Username, "user-")
	var gw string
	switch {
	case strings.HasPrefix(raw, "disp_"):
		gw = "disp.oxylabs.io"
	case strings.HasPrefix(raw, "isp_"):
		gw = "isp.oxylabs.io"
	case strings.HasPrefix(raw, "ddc_"):
		gw = "ddc.oxylabs.io"
	}
	if gw != "" && up.Port >= 8000 && up.Port <= 8999 && net.ParseIP(up.Host) != nil {
		assignedIP := up.Host
		up.Host = gw
		up.Username = "user-" + raw
		return up, assignedIP
	}
	host := strings.ToLower(up.Host)
	if strings.HasSuffix(host, ".oxylabs.io") && !strings.HasPrefix(up.Username, "user-") && !strings.HasPrefix(up.Username, "customer-") {
		if host == "pr.oxylabs.io" {
			up.Username = "customer-" + up.Username
		} else {
			up.Username = "user-" + up.Username
		}
	}
	return up, ""
}
