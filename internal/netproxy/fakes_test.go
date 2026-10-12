// SPDX-License-Identifier: AGPL-3.0-or-later

package netproxy

import (
	"bufio"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"
)

const testUser, testPass = "user__cr.us", "p:ss"

// fakeHTTPProxy is an authenticating upstream that supports CONNECT and absolute-URI requests.
func fakeHTTPProxy(t *testing.T) (Upstream, *atomic.Int32) {
	t.Helper()
	var denied atomic.Int32
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte(testUser+":"+testPass))
	ln := listen(t)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				br := bufio.NewReader(c)
				for {
					req, err := http.ReadRequest(br)
					if err != nil {
						return
					}
					if req.Header.Get("Proxy-Authorization") != want {
						denied.Add(1)
						io.WriteString(c, "HTTP/1.1 407 Proxy Authentication Required\r\nContent-Length: 0\r\n\r\n")
						return
					}
					if req.Method == http.MethodConnect {
						t2, err := net.Dial("tcp", req.Host)
						if err != nil {
							return
						}
						io.WriteString(c, "HTTP/1.1 200 OK\r\n\r\n")
						go io.Copy(t2, br)
						io.Copy(c, t2)
						return
					}
					req.RequestURI = ""
					req.Header.Del("Proxy-Authorization")
					resp, err := http.DefaultTransport.RoundTrip(req)
					if err != nil {
						return
					}
					resp.Write(c)
					resp.Body.Close()
				}
			}()
		}
	}()
	return upstreamFor(ln, "http"), &denied
}

// fakeSOCKS5 implements RFC 1928/1929 with username/password, CONNECT only.
func fakeSOCKS5(t *testing.T) Upstream {
	t.Helper()
	ln := listen(t)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				b := make([]byte, 512)
				io.ReadFull(c, b[:2])
				io.ReadFull(c, b[:b[1]])
				c.Write([]byte{5, 2})
				io.ReadFull(c, b[:2])
				user := make([]byte, b[1])
				io.ReadFull(c, user)
				io.ReadFull(c, b[:1])
				pass := make([]byte, b[0])
				io.ReadFull(c, pass)
				if string(user) != testUser || string(pass) != testPass {
					c.Write([]byte{1, 1})
					return
				}
				c.Write([]byte{1, 0})
				io.ReadFull(c, b[:4])
				var host string
				switch b[3] {
				case 1:
					io.ReadFull(c, b[:4])
					host = net.IP(b[:4]).String()
				case 3:
					io.ReadFull(c, b[:1])
					h := make([]byte, b[0])
					io.ReadFull(c, h)
					host = string(h)
				default:
					return
				}
				io.ReadFull(c, b[:2])
				port := binary.BigEndian.Uint16(b[:2])
				t2, err := net.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(int(port))))
				if err != nil {
					c.Write([]byte{5, 1, 0, 1, 0, 0, 0, 0, 0, 0})
					return
				}
				defer t2.Close()
				c.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
				go io.Copy(t2, c)
				io.Copy(c, t2)
			}()
		}
	}()
	return upstreamFor(ln, "socks5")
}

func listen(t *testing.T) net.Listener {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	return ln
}

func upstreamFor(ln net.Listener, scheme string) Upstream {
	return Upstream{Scheme: scheme, Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port, Username: testUser, Password: testPass}
}
