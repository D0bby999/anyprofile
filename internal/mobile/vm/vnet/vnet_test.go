// SPDX-License-Identifier: AGPL-3.0-or-later

package vnet

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/link/ethernet"
	"gvisor.dev/gvisor/pkg/tcpip/network/arp"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

const testMAC = "52:54:00:12:34:56"

// guest is a second gVisor stack standing in for Android: it joins the
// network over the unix socket with QEMU's framing (4-byte length, frame).
type guest struct {
	s  *stack.Stack
	ch *channel.Endpoint
	c  net.Conn
}

func newGuest(t *testing.T, socket string) *guest {
	t.Helper()
	mac, _ := net.ParseMAC(testMAC)
	ch := channel.New(512, mtu, tcpip.LinkAddress(mac))
	s := stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol, arp.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol, icmp.NewProtocol4},
	})
	if err := s.CreateNIC(1, ethernet.New(ch)); err != nil {
		t.Fatal(err)
	}
	if err := s.AddProtocolAddress(1, tcpip.ProtocolAddress{Protocol: ipv4.ProtocolNumber,
		AddressWithPrefix: tcpip.AddressWithPrefix{Address: addr4(GuestIP), PrefixLen: 24}}, stack.AddressProperties{}); err != nil {
		t.Fatal(err)
	}
	s.SetRouteTable([]tcpip.Route{{Destination: header.IPv4EmptySubnet, Gateway: addr4(GatewayIP), NIC: 1}})
	c, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	g := &guest{s: s, ch: ch, c: c}
	go g.out()
	go g.in()
	t.Cleanup(func() { c.Close(); ch.Close(); s.Close() })
	return g
}

// out sends the guest's frames to the network.
func (g *guest) out() {
	for {
		pkt := g.ch.ReadContext(context.Background())
		if pkt == nil {
			return
		}
		b := pkt.ToView().AsSlice()
		pkt.DecRef()
		frame := make([]byte, 4+len(b))
		binary.BigEndian.PutUint32(frame, uint32(len(b)))
		copy(frame[4:], b)
		if _, err := g.c.Write(frame); err != nil {
			return
		}
	}
}

// in delivers the network's frames to the guest.
func (g *guest) in() {
	r := bufio.NewReader(g.c)
	for {
		var k uint32
		if err := binary.Read(r, binary.BigEndian, &k); err != nil {
			return
		}
		b := make([]byte, k)
		if _, err := io.ReadFull(r, b); err != nil {
			return
		}
		pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{Payload: buffer.MakeWithData(b)})
		g.ch.InjectInbound(0, pkt)
		pkt.DecRef()
	}
}

func (g *guest) dialTCP(ctx context.Context, addr string) (net.Conn, error) {
	host, port, _ := net.SplitHostPort(addr)
	p, _ := net.LookupPort("tcp", port)
	return gonet.DialContextTCP(ctx, g.s, tcpip.FullAddress{NIC: 1, Addr: addr4(host), Port: uint16(p)}, ipv4.ProtocolNumber)
}

// recorder is a Dialer that records targets and connects them to handler.
type recorder struct {
	mu      sync.Mutex
	targets []string
	fail    error
	handler func(net.Conn)
}

func (r *recorder) dial(ctx context.Context, addr string) (net.Conn, error) {
	r.mu.Lock()
	r.targets = append(r.targets, addr)
	fail, h := r.fail, r.handler
	r.mu.Unlock()
	if fail != nil {
		return nil, fail
	}
	a, b := net.Pipe()
	go h(b)
	return a, nil
}

func (r *recorder) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.targets...)
}

func echo(c net.Conn) { defer c.Close(); io.Copy(c, c) }

func start(t *testing.T, d Dialer) (*Network, string) {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "net.sock")
	n, err := Start(Config{Dial: d, MAC: testMAC, Socket: sock})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { n.Close() })
	return n, sock
}

func TestNoDialer(t *testing.T) {
	if _, err := Start(Config{MAC: testMAC, Socket: filepath.Join(t.TempDir(), "s")}); err == nil {
		t.Fatal("a network without a dialer started")
	}
}

func TestTCPGoesThroughDialer(t *testing.T) {
	rec := &recorder{handler: echo}
	_, sock := start(t, rec.dial)
	g := newGuest(t, sock)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := g.dialTCP(ctx, "93.184.216.34:443")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 5)
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.ReadFull(c, buf); err != nil || string(buf) != "hello" {
		t.Fatalf("echo = %q, %v", buf, err)
	}
	if got := rec.seen(); len(got) != 1 || got[0] != "93.184.216.34:443" {
		t.Fatalf("dialed %v", got)
	}
}

func TestDialFailureIsRefused(t *testing.T) {
	rec := &recorder{fail: errors.New("proxy down")}
	_, sock := start(t, rec.dial)
	g := newGuest(t, sock)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if c, err := g.dialTCP(ctx, "93.184.216.34:80"); err == nil {
		c.Close()
		t.Fatal("connected although the proxy is down")
	}
}

func TestInternalAddressesNotDialed(t *testing.T) {
	rec := &recorder{handler: echo}
	_, sock := start(t, rec.dial)
	g := newGuest(t, sock)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, a := range []string{"169.254.169.254:80", GatewayIP + ":8080"} {
		if c, err := g.dialTCP(ctx, a); err == nil {
			c.Close()
			t.Errorf("%s: connected", a)
		}
	}
	if got := rec.seen(); len(got) != 0 {
		t.Fatalf("dialed %v", got)
	}
}

func TestUDPHasNoWayOut(t *testing.T) {
	rec := &recorder{handler: echo}
	_, sock := start(t, rec.dial)
	g := newGuest(t, sock)
	uc, err := gonet.DialUDP(g.s, nil, &tcpip.FullAddress{NIC: 1, Addr: addr4("8.8.8.8"), Port: 53}, ipv4.ProtocolNumber)
	if err != nil {
		t.Fatal(err)
	}
	defer uc.Close()
	uc.Write([]byte{0, 1, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0})
	uc.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := uc.Read(make([]byte, 512)); err == nil {
		t.Fatal("a UDP answer came back from 8.8.8.8")
	}
	if got := rec.seen(); len(got) != 0 {
		t.Fatalf("dialed %v", got)
	}
}

// query is a DNS question for example.com A, id 0x1234.
var query = []byte{0x12, 0x34, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0, 7, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 3, 'c', 'o', 'm', 0, 0, 1, 0, 1}

func TestDNSOverDoH(t *testing.T) {
	answer := append([]byte(nil), query...)
	answer[2] = 0x81 // a response
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q, _ := io.ReadAll(r.Body)
		if r.Header.Get("Content-Type") != "application/dns-message" || string(q) != string(query) {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/dns-message")
		w.Write(answer)
	}))
	srv.StartTLS()
	defer srv.Close()
	dial := func(ctx context.Context, addr string) (net.Conn, error) {
		if addr != "1.1.1.1:443" {
			return nil, errors.New("only DoH here")
		}
		return (&net.Dialer{}).DialContext(ctx, "tcp", srv.Listener.Addr().String())
	}
	sock := filepath.Join(t.TempDir(), "net.sock")
	n, err := Start(Config{Dial: dial, MAC: testMAC, Socket: sock})
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()
	// The test server's certificate is not Cloudflare's.
	n.dnsTLS(&tls.Config{InsecureSkipVerify: true})
	g := newGuest(t, sock)
	uc, err := gonet.DialUDP(g.s, nil, &tcpip.FullAddress{NIC: 1, Addr: addr4(GatewayIP), Port: 53}, ipv4.ProtocolNumber)
	if err != nil {
		t.Fatal(err)
	}
	defer uc.Close()
	uc.Write(query)
	uc.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 512)
	k, err := uc.Read(buf)
	if err != nil || string(buf[:k]) != string(answer) {
		t.Fatalf("answer %x, %v", buf[:k], err)
	}
}

func TestDNSFallsBackToTCP(t *testing.T) {
	answer := append([]byte(nil), query...)
	answer[2] = 0x81
	dial := func(ctx context.Context, addr string) (net.Conn, error) {
		if addr != tcpResolver {
			return nil, errors.New("port 443 blocked by the proxy")
		}
		a, b := net.Pipe()
		go func() {
			defer b.Close()
			var k uint16
			binary.Read(b, binary.BigEndian, &k)
			io.ReadFull(b, make([]byte, k))
			out := make([]byte, 2+len(answer))
			binary.BigEndian.PutUint16(out, uint16(len(answer)))
			copy(out[2:], answer)
			b.Write(out)
		}()
		return a, nil
	}
	r := newResolver(dial)
	got, err := r.exchange(context.Background(), query)
	if err != nil || string(got) != string(answer) {
		t.Fatalf("%x, %v", got, err)
	}
}

func TestForwardTCPReachesGuest(t *testing.T) {
	n, sock := start(t, (&recorder{handler: echo}).dial)
	g := newGuest(t, sock)
	ln, err := gonet.ListenTCP(g.s, tcpip.FullAddress{NIC: 1, Addr: addr4(GuestIP), Port: 5555}, ipv4.ProtocolNumber)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		if c, err := ln.Accept(); err == nil {
			echo(c)
		}
	}()
	port, err := n.ForwardTCP(5555)
	if err != nil {
		t.Fatal(err)
	}
	c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.Write([]byte("adb"))
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 3)
	if _, err := io.ReadFull(c, buf); err != nil || string(buf) != "adb" {
		t.Fatalf("%q, %v", buf, err)
	}
}

func TestCloseLeavesNothing(t *testing.T) {
	n, sock := start(t, (&recorder{handler: echo}).dial)
	newGuest(t, sock)
	if _, err := n.ForwardTCP(5555); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { n.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close hangs")
	}
	if _, err := net.Dial("unix", sock); err == nil {
		t.Fatal("socket still accepts")
	}
}
