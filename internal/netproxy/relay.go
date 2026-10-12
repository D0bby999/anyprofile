// SPDX-License-Identifier: AGPL-3.0-or-later

package netproxy

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	maxConns      = 256              // per relay; a runaway page must not starve the app
	headerTimeout = 30 * time.Second // client must send its request line within this
	idleTimeout   = 10 * time.Minute // tunnel with no traffic either way is closed
)

// Relay is an unauthenticated HTTP proxy on 127.0.0.1 that forwards every
// request to one upstream with credentials. The browser only ever sees the
// loopback address; the password stays in this process.
type Relay struct {
	up    Upstream
	ln    net.Listener
	sem   chan struct{}
	mu    sync.Mutex
	conns map[net.Conn]struct{}
	wg    sync.WaitGroup
}

// StartRelay listens on 127.0.0.1:port (0 picks a free port).
func StartRelay(up Upstream, port int) (*Relay, error) {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, err
	}
	r := &Relay{up: up, ln: ln, sem: make(chan struct{}, maxConns), conns: map[net.Conn]struct{}{}}
	r.wg.Add(1)
	go r.serve()
	return r, nil
}

func (r *Relay) Port() int { return r.ln.Addr().(*net.TCPAddr).Port }

// Close stops listening and drops every open connection.
func (r *Relay) Close() error {
	err := r.ln.Close()
	r.mu.Lock()
	for c := range r.conns {
		c.Close()
	}
	r.mu.Unlock()
	r.wg.Wait()
	return err
}

func (r *Relay) track(c net.Conn, on bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if on {
		r.conns[c] = struct{}{}
	} else {
		delete(r.conns, c)
	}
}

func (r *Relay) serve() {
	defer r.wg.Done()
	for {
		c, err := r.ln.Accept()
		if err != nil {
			return
		}
		if ip := c.RemoteAddr().(*net.TCPAddr).IP; !ip.IsLoopback() {
			c.Close()
			continue
		}
		select {
		case r.sem <- struct{}{}:
		default:
			c.Close() // over the limit
			continue
		}
		r.track(c, true)
		r.wg.Add(1)
		go func() {
			defer func() { r.track(c, false); c.Close(); <-r.sem; r.wg.Done() }()
			r.handle(c)
		}()
	}
}

func (r *Relay) handle(c net.Conn) {
	br := bufio.NewReader(c)
	c.SetReadDeadline(time.Now().Add(headerTimeout))
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	c.SetReadDeadline(time.Time{})
	if req.Method == http.MethodConnect {
		r.tunnel(c, br, req.Host)
		return
	}
	r.forwardHTTP(c, br, req)
}

func (r *Relay) tunnel(c net.Conn, br *bufio.Reader, target string) {
	up, err := r.up.DialTunnel(context.Background(), target)
	if err != nil {
		io.WriteString(c, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n")
		return
	}
	r.track(up, true)
	defer func() { r.track(up, false); up.Close() }()
	if _, err := io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	pipe(&bufConn{Conn: c, r: br}, up)
}

// forwardHTTP handles plain-http requests (absolute URI). For an HTTP upstream
// each request is re-sent with the credential; for SOCKS5 the connection is
// bound to the first request's host.
func (r *Relay) forwardHTTP(c net.Conn, br *bufio.Reader, req *http.Request) {
	var up net.Conn
	var err error
	if r.up.Scheme == "http" {
		var d net.Dialer
		ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
		up, err = d.DialContext(ctx, "tcp", r.up.Addr())
		cancel()
	} else {
		up, err = r.up.DialTunnel(context.Background(), hostPort(req))
	}
	if err != nil {
		io.WriteString(c, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n")
		return
	}
	r.track(up, true)
	defer func() { r.track(up, false); up.Close() }()
	host := req.URL.Host
	go func() { io.Copy(c, up); c.Close() }()
	for {
		req.Header.Del("Proxy-Authorization")
		req.Header.Del("Proxy-Connection")
		if r.up.Scheme == "http" {
			if a := r.up.basicAuth(); a != "" {
				req.Header.Set("Proxy-Authorization", a)
			}
			err = req.WriteProxy(up)
		} else {
			err = req.Write(up)
		}
		if err != nil {
			return
		}
		c.SetReadDeadline(time.Now().Add(idleTimeout))
		if req, err = http.ReadRequest(br); err != nil {
			return
		}
		if r.up.Scheme != "http" && req.URL.Host != host {
			return // a SOCKS tunnel is bound to one host; the browser reconnects
		}
	}
}

func hostPort(req *http.Request) string {
	if req.URL.Port() != "" {
		return req.URL.Host
	}
	return net.JoinHostPort(req.URL.Hostname(), "80")
}
