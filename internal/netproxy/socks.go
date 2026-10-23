// SPDX-License-Identifier: AGPL-3.0-or-later

package netproxy

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

// SOCKSRelay is an unauthenticated SOCKS5 proxy on 127.0.0.1 that forwards
// every CONNECT through one upstream with credentials, as Relay does for
// HTTP. A mobile device reaches it over adb reverse and never sees the
// password. Only CONNECT is served: UDP ASSOCIATE and BIND are refused, so
// UDP has no way out.
type SOCKSRelay struct {
	up Upstream
	ln net.Listener

	mu    sync.Mutex
	conns map[net.Conn]struct{}
	first chan struct{} // closed at the first CONNECT that reached the upstream
	once  sync.Once
	wg    sync.WaitGroup
}

// StartSOCKS listens on 127.0.0.1:port (0 picks a free port).
func StartSOCKS(up Upstream, port int) (*SOCKSRelay, error) {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, err
	}
	r := &SOCKSRelay{up: up, ln: ln, conns: map[net.Conn]struct{}{}, first: make(chan struct{})}
	r.wg.Add(1)
	go r.serve()
	return r, nil
}

func (r *SOCKSRelay) Port() int { return r.ln.Addr().(*net.TCPAddr).Port }

// Used is closed once a connection has gone through: the device's tunnel works.
func (r *SOCKSRelay) Used() <-chan struct{} { return r.first }

func (r *SOCKSRelay) Close() error {
	err := r.ln.Close()
	r.mu.Lock()
	for c := range r.conns {
		c.Close()
	}
	r.mu.Unlock()
	r.wg.Wait()
	return err
}

func (r *SOCKSRelay) track(c net.Conn, on bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if on {
		r.conns[c] = struct{}{}
	} else {
		delete(r.conns, c)
	}
}

func (r *SOCKSRelay) serve() {
	defer r.wg.Done()
	for {
		c, err := r.ln.Accept()
		if err != nil {
			return
		}
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			r.track(c, true)
			defer r.track(c, false)
			defer c.Close()
			r.handle(c)
		}()
	}
}

const (
	socksVer       = 5
	cmdConnect     = 1
	atypIPv4       = 1
	atypDomain     = 3
	atypIPv6       = 4
	repSucceeded   = 0
	repFailure     = 1
	repUnreachable = 4
	repCmdNotSup   = 7
	repAtypNotSup  = 8
)

var errSOCKS = errors.New("socks: bad request")

func (r *SOCKSRelay) handle(c net.Conn) {
	c.SetDeadline(time.Now().Add(30 * time.Second))
	br := bufio.NewReader(c)
	// Greeting: only "no authentication" is offered back.
	var hdr [2]byte
	if _, err := io.ReadFull(br, hdr[:]); err != nil || hdr[0] != socksVer {
		return
	}
	methods := make([]byte, hdr[1])
	if _, err := io.ReadFull(br, methods); err != nil {
		return
	}
	noAuth := false
	for _, m := range methods {
		noAuth = noAuth || m == 0
	}
	if !noAuth {
		c.Write([]byte{socksVer, 0xff})
		return
	}
	c.Write([]byte{socksVer, 0})

	target, cmd, rep := readRequest(br)
	if rep != repSucceeded {
		reply(c, rep)
		return
	}
	if cmd != cmdConnect {
		reply(c, repCmdNotSup)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	out, err := r.up.DialTunnel(ctx, target)
	cancel()
	if err != nil {
		reply(c, repUnreachable)
		return
	}
	r.track(out, true)
	defer r.track(out, false)
	defer out.Close()
	if err := reply(c, repSucceeded); err != nil {
		return
	}
	r.once.Do(func() { close(r.first) })
	c.SetDeadline(time.Time{})
	// br may hold bytes the client sent right after its request.
	pipe(&bufConn{Conn: c, r: br}, out)
}

// readRequest reads a SOCKS5 request: the target as host:port, the command,
// and a reply code when the request cannot be served.
func readRequest(br *bufio.Reader) (target string, cmd byte, rep byte) {
	var h [4]byte
	if _, err := io.ReadFull(br, h[:]); err != nil || h[0] != socksVer {
		return "", 0, repFailure
	}
	var host string
	switch h[3] {
	case atypIPv4:
		b := make([]byte, 4)
		if _, err := io.ReadFull(br, b); err != nil {
			return "", 0, repFailure
		}
		host = net.IP(b).String()
	case atypIPv6:
		b := make([]byte, 16)
		if _, err := io.ReadFull(br, b); err != nil {
			return "", 0, repFailure
		}
		host = net.IP(b).String()
	case atypDomain:
		n, err := br.ReadByte()
		if err != nil || n == 0 {
			return "", 0, repFailure
		}
		b := make([]byte, n)
		if _, err := io.ReadFull(br, b); err != nil {
			return "", 0, repFailure
		}
		host = string(b)
	default:
		return "", 0, repAtypNotSup
	}
	var p [2]byte
	if _, err := io.ReadFull(br, p[:]); err != nil {
		return "", 0, repFailure
	}
	return net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(p[:])))), h[1], repSucceeded
}

// reply answers with a zero bind address; clients do not use it for CONNECT.
func reply(c net.Conn, rep byte) error {
	_, err := c.Write([]byte{socksVer, rep, 0, atypIPv4, 0, 0, 0, 0, 0, 0})
	return err
}
