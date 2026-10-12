// SPDX-License-Identifier: AGPL-3.0-or-later

package netproxy

import (
	"net"
	"sync/atomic"
	"time"
)

// pipe copies both ways until either side closes, or until neither direction
// has moved a byte for idleTimeout (a one-way download keeps the tunnel alive).
func pipe(a, b net.Conn) {
	var last atomic.Int64
	last.Store(time.Now().UnixNano())
	done := make(chan struct{}, 2)
	cp := func(dst, src net.Conn) {
		buf := make([]byte, 32*1024)
		for {
			src.SetReadDeadline(time.Now().Add(idleTimeout))
			n, err := src.Read(buf)
			if n > 0 {
				last.Store(time.Now().UnixNano())
				if _, werr := dst.Write(buf[:n]); werr != nil {
					break
				}
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() &&
				time.Since(time.Unix(0, last.Load())) < idleTimeout {
				continue // this side is quiet, the other is not
			}
			if err != nil {
				break
			}
		}
		if tc, ok := dst.(interface{ CloseWrite() error }); ok {
			tc.CloseWrite()
		} else {
			dst.Close()
		}
		done <- struct{}{}
	}
	go cp(a, b)
	go cp(b, a)
	<-done
	// One side finished; give the other a moment to drain, then tear down.
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
	a.Close()
	b.Close()
}
