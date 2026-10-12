// SPDX-License-Identifier: AGPL-3.0-or-later

// Package netproxy runs the per-profile loopback relay and inspects upstream proxies.
package netproxy

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/net/proxy"
)

const dialTimeout = 20 * time.Second

// Upstream is a proxy the relay forwards to. Password is filled from the
// Keychain just before use and must never be logged; use Redacted.
type Upstream struct {
	Scheme   string // "http" | "socks5"
	Host     string
	Port     int
	Username string
	Password string
}

func (u Upstream) Addr() string { return net.JoinHostPort(u.Host, strconv.Itoa(u.Port)) }

// Redacted is the loggable form: scheme://user@host:port.
func (u Upstream) Redacted() string {
	user := ""
	if u.Username != "" {
		user = u.Username + "@"
	}
	return u.Scheme + "://" + user + u.Addr()
}

// URL carries the credential; only hand it to an in-process http.Transport.
func (u Upstream) URL() *url.URL {
	v := &url.URL{Scheme: u.Scheme, Host: u.Addr()}
	if u.Username != "" || u.Password != "" {
		v.User = url.UserPassword(u.Username, u.Password)
	}
	return v
}

func (u Upstream) basicAuth() string {
	if u.Username == "" && u.Password == "" {
		return ""
	}
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(u.Username+":"+u.Password))
}

// bufConn keeps bytes the upstream sent after its CONNECT reply.
type bufConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufConn) Read(p []byte) (int, error) { return c.r.Read(p) }

var ErrUpstreamAuth = errors.New("upstream proxy rejected the credentials (407)")

// DialTunnel opens a TCP stream to target ("host:port") through the upstream.
func (u Upstream) DialTunnel(ctx context.Context, target string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	switch u.Scheme {
	case "socks5":
		var auth *proxy.Auth
		if u.Username != "" || u.Password != "" {
			auth = &proxy.Auth{User: u.Username, Password: u.Password}
		}
		d, err := proxy.SOCKS5("tcp", u.Addr(), auth, &net.Dialer{})
		if err != nil {
			return nil, err
		}
		return d.(proxy.ContextDialer).DialContext(ctx, "tcp", target)
	case "http":
		var d net.Dialer
		c, err := d.DialContext(ctx, "tcp", u.Addr())
		if err != nil {
			return nil, err
		}
		if dl, ok := ctx.Deadline(); ok {
			c.SetDeadline(dl)
		}
		req := "CONNECT " + target + " HTTP/1.1\r\nHost: " + target + "\r\n"
		if a := u.basicAuth(); a != "" {
			req += "Proxy-Authorization: " + a + "\r\n"
		}
		if _, err := c.Write([]byte(req + "\r\n")); err != nil {
			c.Close()
			return nil, err
		}
		br := bufio.NewReader(c)
		resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
		if err != nil {
			c.Close()
			return nil, err
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusProxyAuthRequired {
			c.Close()
			return nil, ErrUpstreamAuth
		}
		if resp.StatusCode != http.StatusOK {
			c.Close()
			return nil, fmt.Errorf("upstream CONNECT %s: %s", target, resp.Status)
		}
		c.SetDeadline(time.Time{})
		return &bufConn{Conn: c, r: br}, nil
	}
	return nil, fmt.Errorf("unsupported proxy scheme %q", u.Scheme)
}
