// SPDX-License-Identifier: AGPL-3.0-or-later
package service

import (
	"context"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/netproxy"
)

const (
	// proxyWatchEvery is how often the proxy of each running device is tried.
	proxyWatchEvery = 2 * time.Minute
	// proxyWatchMisses failed tries in a row mark the proxy failed, so one
	// slow answer does not.
	proxyWatchMisses = 2
	// proxyPingTarget is reached through the proxy; only the TCP connection
	// is made, nothing is sent.
	proxyPingTarget = "1.1.1.1:443"
)

// pingProxy opens one connection through up and closes it.
func pingProxy(ctx context.Context, up netproxy.Upstream) error {
	c, err := up.DialTunnel(ctx, proxyPingTarget)
	if err != nil {
		return err
	}
	return c.Close()
}

// WatchMobileProxies tries the proxy of every running mobile device until
// ctx ends. A device whose proxy stops answering has no network at all (it
// never falls back to the real IP), so the proxy is marked failed and the
// Mobile page says so; once it answers again, a full check clears the mark.
func (s *Service) WatchMobileProxies(ctx context.Context) {
	misses := map[string]int{}
	t := time.NewTicker(proxyWatchEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.watchMobileProxies(ctx, misses)
		}
	}
}

// watchMobileProxies is one round; misses counts failed tries per proxy.
func (s *Service) watchMobileProxies(ctx context.Context, misses map[string]int) {
	if s.Mobile == nil {
		return
	}
	ping := s.PingProxy
	if ping == nil {
		ping = pingProxy
	}
	seen := map[string]bool{}
	for _, in := range s.Mobile.List() {
		if in.State != mobile.Running {
			continue
		}
		d, err := s.Store.GetMobileDevice(in.DeviceID)
		if err != nil || d.ProxyID == "" || seen[d.ProxyID] {
			continue
		}
		id := d.ProxyID
		seen[id] = true
		px, err := s.Store.GetProxy(id)
		if err != nil {
			continue
		}
		up, err := s.proxyUpstream(id)
		if err != nil {
			continue
		}
		if err := ping(ctx, up); err != nil {
			misses[id]++
			if misses[id] == proxyWatchMisses && px.FailedAt == "" {
				s.Log.Warn("mobile proxy not answering", "proxy", id, "err", err)
				if s.Store.SaveProxyFailure(id) == nil {
					s.notify("proxies")
				}
			}
			continue
		}
		misses[id] = 0
		if px.FailedAt != "" {
			_, _ = s.CheckProxyByID(ctx, id)
		}
	}
	for id := range misses {
		if !seen[id] {
			delete(misses, id)
		}
	}
}
