// SPDX-License-Identifier: AGPL-3.0-or-later

package vnet

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
)

// The VM asks the gateway (DHCP hands it out as the DNS server). Each query
// goes, as wire bytes, to a public resolver through Dial: DNS over HTTPS on
// 443 first, since many HTTP proxies only CONNECT to 80 and 443, then DNS
// over TCP on 53. The host's own resolver is never asked.
const (
	dohURL       = "https://1.1.1.1/dns-query"
	dohHost      = "cloudflare-dns.com"
	tcpResolver  = "1.1.1.1:53"
	queryTimeout = 8 * time.Second
	maxDNSMsg    = 65535
)

type resolver struct {
	dial   Dialer
	client *http.Client
}

func newResolver(dial Dialer) *resolver {
	tr := &http.Transport{
		DialContext:         func(ctx context.Context, _, addr string) (net.Conn, error) { return dial(ctx, addr) },
		TLSClientConfig:     &tls.Config{ServerName: dohHost, MinVersion: tls.VersionTLS12},
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        2,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: queryTimeout,
	}
	return &resolver{dial: dial, client: &http.Client{Transport: tr, Timeout: queryTimeout}}
}

// exchange answers one DNS message.
func (r *resolver) exchange(ctx context.Context, q []byte) ([]byte, error) {
	ans, err := r.doh(ctx, q)
	if err == nil {
		return ans, nil
	}
	ans, err2 := r.overTCP(ctx, q)
	if err2 == nil {
		return ans, nil
	}
	return nil, errors.Join(err, err2)
}

func (r *resolver) doh(ctx context.Context, q []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dohURL, bytes.NewReader(q))
	if err != nil {
		return nil, err
	}
	req.Host = dohHost
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("doh: %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxDNSMsg))
}

func (r *resolver) overTCP(ctx context.Context, q []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	c, err := r.dial(ctx, tcpResolver)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	if dl, ok := ctx.Deadline(); ok {
		c.SetDeadline(dl)
	}
	return tcpExchange(c, q)
}

// tcpExchange writes q with DNS-over-TCP framing and reads one framed answer.
func tcpExchange(c io.ReadWriter, q []byte) ([]byte, error) {
	if len(q) > maxDNSMsg {
		return nil, errors.New("dns message too long")
	}
	msg := make([]byte, 2+len(q))
	binary.BigEndian.PutUint16(msg, uint16(len(q)))
	copy(msg[2:], q)
	if _, err := c.Write(msg); err != nil {
		return nil, err
	}
	var n uint16
	if err := binary.Read(c, binary.BigEndian, &n); err != nil {
		return nil, err
	}
	ans := make([]byte, n)
	_, err := io.ReadFull(c, ans)
	return ans, err
}

// dnsTLS replaces the DoH client's TLS settings; tests use it.
func (n *Network) dnsTLS(c *tls.Config) {
	n.dns.client.Transport.(*http.Transport).TLSClientConfig = c
}

// serveDNS answers on the gateway's port 53, UDP and TCP.
func (n *Network) serveDNS() error {
	r := newResolver(n.cfg.Dial)
	n.dns = r
	at := tcpip.FullAddress{NIC: 1, Addr: addr4(GatewayIP), Port: 53}
	uc, err := gonet.DialUDP(n.stack, &at, nil, ipv4.ProtocolNumber)
	if err != nil {
		return err
	}
	n.track(uc)
	tl, err := gonet.ListenTCP(n.stack, at, ipv4.ProtocolNumber)
	if err != nil {
		return err
	}
	n.track(tl)
	n.goServe(func() { n.serveDNSUDP(uc, r) })
	n.goServe(func() { n.serveDNSTCP(tl, r) })
	return nil
}

func (n *Network) serveDNSUDP(uc *gonet.UDPConn, r *resolver) {
	buf := make([]byte, maxDNSMsg)
	for {
		k, from, err := uc.ReadFrom(buf)
		if err != nil {
			return
		}
		q := append([]byte(nil), buf[:k]...)
		go func() {
			ctx, cancel := context.WithTimeout(n.ctx, 2*queryTimeout)
			defer cancel()
			if ans, err := r.exchange(ctx, q); err == nil {
				_, _ = uc.WriteTo(ans, from)
			}
		}()
	}
}

func (n *Network) serveDNSTCP(tl *gonet.TCPListener, r *resolver) {
	for {
		c, err := tl.Accept()
		if err != nil {
			return
		}
		go func() {
			defer c.Close()
			for {
				c.SetDeadline(time.Now().Add(30 * time.Second))
				var k uint16
				if err := binary.Read(c, binary.BigEndian, &k); err != nil {
					return
				}
				q := make([]byte, k)
				if _, err := io.ReadFull(c, q); err != nil {
					return
				}
				ctx, cancel := context.WithTimeout(n.ctx, 2*queryTimeout)
				ans, err := r.exchange(ctx, q)
				cancel()
				if err != nil {
					return
				}
				out := make([]byte, 2+len(ans))
				binary.BigEndian.PutUint16(out, uint16(len(ans)))
				copy(out[2:], ans)
				if _, err := c.Write(out); err != nil {
					return
				}
			}
		}()
	}
}
