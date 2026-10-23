// SPDX-License-Identifier: AGPL-3.0-or-later

// Package vnet is the network of one Android VM: a userspace TCP/IP stack
// (gVisor, through the building blocks of gvisor-tap-vsock) that QEMU joins
// over a unix socket. Every TCP connection the VM opens is dialed by Dial,
// which in AnyProfile goes through the device's proxy. DNS is answered on the
// gateway and resolved through Dial too. UDP and ICMP have no way out, and
// the stack has no IPv6, so nothing leaves by the host's own route.
package vnet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/containers/gvisor-tap-vsock/pkg/services/dhcp"
	"github.com/containers/gvisor-tap-vsock/pkg/tap"
	"github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/inetaf/tcpproxy"
	"github.com/sirupsen/logrus"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/network/arp"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

const (
	Subnet    = "192.168.127.0/24"
	GatewayIP = "192.168.127.1"
	// GuestIP is the one address the VM gets, by a static DHCP lease on its MAC.
	GuestIP    = "192.168.127.2"
	gatewayMAC = "5a:94:ef:e4:0c:dd"
	mtu        = 1500

	dialTimeout = 30 * time.Second
	maxInFlight = 256
)

// Dialer opens a TCP stream to addr ("host:port"). It must not fall back to
// another route: an error is a connection the VM does not get.
type Dialer func(ctx context.Context, addr string) (net.Conn, error)

// Config is one VM's network.
type Config struct {
	Dial Dialer
	// MAC is the VM's network card, as QEMU is told; it gets GuestIP.
	MAC string
	// Socket is the unix socket QEMU connects to (-netdev stream, client).
	Socket string
}

// Network is a running VM network. Close ends it.
type Network struct {
	cfg   Config
	stack *stack.Stack
	sw    *tap.Switch
	ln    net.Listener
	ctx   context.Context
	stop  context.CancelFunc
	dns   *resolver

	mu      sync.Mutex
	closers []io.Closer
	wg      sync.WaitGroup
}

func init() {
	// The DHCP server and switch log every packet at Info; the app log keeps
	// warnings only.
	logrus.SetLevel(logrus.WarnLevel)
}

// Start builds the stack and listens on cfg.Socket for QEMU.
func Start(cfg Config) (*Network, error) {
	if cfg.Dial == nil {
		return nil, errors.New("vnet: no dialer: the VM must not get a direct route by accident")
	}
	if _, err := net.ParseMAC(cfg.MAC); err != nil {
		return nil, fmt.Errorf("vnet: mac: %w", err)
	}
	conf := &types.Configuration{
		MTU: mtu, Subnet: Subnet, GatewayIP: GatewayIP, GatewayMacAddress: gatewayMAC,
		DHCPStaticLeases: map[string]string{GuestIP: cfg.MAC},
	}
	pool, err := tap.NewIPPool(Subnet)
	if err != nil {
		return nil, err
	}
	if err := pool.Reserve(GatewayIP, gatewayMAC); err != nil {
		return nil, err
	}
	if err := pool.Reserve(GuestIP, cfg.MAC); err != nil {
		return nil, err
	}
	link, err := tap.NewLinkEndpoint(false, mtu, gatewayMAC, GatewayIP, nil)
	if err != nil {
		return nil, err
	}
	sw := tap.NewSwitch(false)
	link.Connect(sw)
	sw.Connect(link)

	s, err := newStack(link)
	if err != nil {
		return nil, err
	}
	ctx, stop := context.WithCancel(context.Background())
	n := &Network{cfg: cfg, stack: s, sw: sw, ctx: ctx, stop: stop}

	s.SetTransportProtocolHandler(tcp.ProtocolNumber, n.tcpForwarder().HandlePacket)
	// Bound endpoints (DHCP on 67, DNS on 53 of the gateway) take precedence
	// over these handlers, so only the way out is closed.
	drop := func(stack.TransportEndpointID, *stack.PacketBuffer) bool { return true }
	s.SetTransportProtocolHandler(udp.ProtocolNumber, drop)
	s.SetTransportProtocolHandler(icmp.ProtocolNumber4, drop)

	srv, err := dhcp.New(conf, s, pool)
	if err != nil {
		n.Close()
		return nil, err
	}
	n.goServe(func() { _ = srv.Serve() })
	if err := n.serveDNS(); err != nil {
		n.Close()
		return nil, err
	}

	_ = os.Remove(cfg.Socket)
	ln, err := net.Listen("unix", cfg.Socket)
	if err != nil {
		n.Close()
		return nil, err
	}
	if err := os.Chmod(cfg.Socket, 0o600); err != nil {
		ln.Close()
		n.Close()
		return nil, err
	}
	n.ln = ln
	n.goServe(n.acceptQemu)
	return n, nil
}

func newStack(link stack.LinkEndpoint) (*stack.Stack, error) {
	s := stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol, arp.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol, icmp.NewProtocol4},
	})
	if err := s.CreateNIC(1, link); err != nil {
		return nil, errors.New(err.String())
	}
	if err := s.AddProtocolAddress(1, tcpip.ProtocolAddress{
		Protocol:          ipv4.ProtocolNumber,
		AddressWithPrefix: addr4(GatewayIP).WithPrefix(),
	}, stack.AddressProperties{}); err != nil {
		return nil, errors.New(err.String())
	}
	// The gateway answers for every destination: that is how the VM's
	// connections to the internet reach the forwarder.
	s.SetSpoofing(1, true)
	s.SetPromiscuousMode(1, true)
	_, cidr, _ := net.ParseCIDR(Subnet)
	sub, err := tcpip.NewSubnet(tcpip.AddrFromSlice(cidr.IP), tcpip.MaskFromBytes(cidr.Mask))
	if err != nil {
		return nil, err
	}
	s.SetRouteTable([]tcpip.Route{{Destination: sub, NIC: 1}})
	return s, nil
}

func addr4(ip string) tcpip.Address { return tcpip.AddrFrom4Slice(net.ParseIP(ip).To4()) }

func (n *Network) goServe(fn func()) {
	n.wg.Add(1)
	go func() { defer n.wg.Done(); fn() }()
}

func (n *Network) track(c io.Closer) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.closers = append(n.closers, c)
}

// acceptQemu serves QEMU; when QEMU reconnects (after the app restarts and
// adopts the VM) the new connection joins the same switch.
func (n *Network) acceptQemu() {
	for {
		c, err := n.ln.Accept()
		if err != nil {
			return
		}
		n.goServe(func() {
			defer c.Close()
			_ = n.sw.Accept(n.ctx, c, types.QemuProtocol)
		})
	}
}

// internal is an address the VM must not reach through the proxy: this
// network itself and link-local (cloud metadata).
func internal(a tcpip.Address) bool {
	ip := net.IP(a.AsSlice())
	_, sub, _ := net.ParseCIDR(Subnet)
	return sub.Contains(ip) || ip.IsLinkLocalUnicast() || ip.IsLoopback() || ip.IsUnspecified()
}

func (n *Network) tcpForwarder() *tcp.Forwarder {
	return tcp.NewForwarder(n.stack, 0, maxInFlight, func(r *tcp.ForwarderRequest) {
		id := r.ID()
		if internal(id.LocalAddress) {
			r.Complete(true) // RST
			return
		}
		target := net.JoinHostPort(id.LocalAddress.String(), strconv.Itoa(int(id.LocalPort)))
		ctx, cancel := context.WithTimeout(n.ctx, dialTimeout)
		out, err := n.cfg.Dial(ctx, target)
		cancel()
		if err != nil {
			r.Complete(true)
			return
		}
		var wq waiter.Queue
		ep, tcpErr := r.CreateEndpoint(&wq)
		r.Complete(false)
		if tcpErr != nil {
			out.Close()
			return
		}
		in := gonet.NewTCPConn(&wq, ep)
		proxy := tcpproxy.DialProxy{DialContext: func(context.Context, string, string) (net.Conn, error) { return out, nil }}
		proxy.HandleConn(in)
	})
}

// ForwardTCP listens on 127.0.0.1 (a free port) and connects each client to
// the VM's guestPort. It is how adb reaches the VM. The listener closes with
// the network.
func (n *Network) ForwardTCP(guestPort int) (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	n.track(ln)
	guest := tcpip.FullAddress{NIC: 1, Addr: addr4(GuestIP), Port: uint16(guestPort)}
	n.goServe(func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				ctx, cancel := context.WithTimeout(n.ctx, 10*time.Second)
				g, err := gonet.DialContextTCP(ctx, n.stack, guest, ipv4.ProtocolNumber)
				cancel()
				if err != nil {
					c.Close()
					return
				}
				proxy := tcpproxy.DialProxy{DialContext: func(context.Context, string, string) (net.Conn, error) { return g, nil }}
				proxy.HandleConn(c)
			}()
		}
	})
	return ln.Addr().(*net.TCPAddr).Port, nil
}

// Close ends the network: QEMU loses its link, every forwarded connection
// drops, and no goroutine is left.
func (n *Network) Close() error {
	n.stop()
	if n.ln != nil {
		n.ln.Close()
		os.Remove(n.cfg.Socket)
	}
	n.mu.Lock()
	for _, c := range n.closers {
		c.Close()
	}
	n.closers = nil
	n.mu.Unlock()
	n.stack.Close()
	n.wg.Wait()
	return nil
}
