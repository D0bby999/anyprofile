// SPDX-License-Identifier: AGPL-3.0-or-later

package netproxy

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ParsedLine is one line of a pasted proxy list.
type ParsedLine struct {
	Line     int      `json:"line"`
	Upstream Upstream `json:"-"`
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
//	scheme://user:pass@host:port   (scheme http, https→http, socks5, socks5h)
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
		p := ParsedLine{Line: i + 1, Upstream: up}
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

func parseOne(line string) (Upstream, error) {
	if strings.Contains(line, "://") {
		u, err := url.Parse(line)
		if err != nil {
			return Upstream{}, errf("not_url", "not a proxy URL")
		}
		scheme := map[string]string{"http": "http", "https": "http", "socks5": "socks5", "socks5h": "socks5"}[u.Scheme]
		if scheme == "" {
			return Upstream{}, errf("scheme", "unsupported scheme %q", u.Scheme)
		}
		port, err := strconv.Atoi(u.Port())
		if err != nil || u.Hostname() == "" {
			return Upstream{}, errf("host_port", "needs host:port")
		}
		pass, _ := u.User.Password()
		return check(Upstream{Scheme: scheme, Host: u.Hostname(), Port: port, Username: u.User.Username(), Password: pass})
	}
	parts := strings.Split(line, ":")
	if len(parts) != 2 && len(parts) < 4 {
		return Upstream{}, errf("format", "expected host:port or host:port:user:pass")
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return Upstream{}, errf("port", "bad port %q", parts[1])
	}
	up := Upstream{Scheme: "http", Host: parts[0], Port: port}
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
