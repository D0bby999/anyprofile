// SPDX-License-Identifier: AGPL-3.0-or-later

package netproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Exit is where an upstream proxy leaves onto the internet, measured through it.
type Exit struct {
	IP        string   `json:"ip"`
	Country   string   `json:"country"`
	City      string   `json:"city"`
	Timezone  string   `json:"timezone"`
	Lat       *float64 `json:"lat"`
	Lon       *float64 `json:"lon"`
	LatencyMs int64    `json:"latencyMs"`
	Rotating  bool     `json:"rotating"` // the exit IP changed between probes
	Source    string   `json:"source"`
}

// Geo lookups always go through the proxy (never from the real IP) and always
// verify TLS. Overridable in tests.
var (
	ipinfoURL = "https://ipinfo.io/json"
	ipwhoURL  = "https://ipwho.is/"
	ipifyURL  = "https://api.ipify.org"
)

func clientVia(up Upstream, keepAlive bool) *http.Client {
	return &http.Client{
		Timeout: 25 * time.Second,
		Transport: &http.Transport{
			Proxy:             http.ProxyURL(up.URL()),
			DisableKeepAlives: !keepAlive,
			ForceAttemptHTTP2: true,
		},
	}
}

// Check measures the exit of up: GeoIP from ipinfo (ipwho.is as fallback),
// a normalized timezone, latency, and whether the IP rotates between connections.
func Check(ctx context.Context, up Upstream) (Exit, error) {
	t0 := time.Now()
	e, err := geoIPInfo(ctx, clientVia(up, true))
	if err != nil {
		var err2 error
		if e, err2 = geoIPWho(ctx, clientVia(up, true)); err2 != nil {
			return Exit{}, fmt.Errorf("geoip through proxy: %v; fallback: %v", err, err2)
		}
	}
	e.LatencyMs = time.Since(t0).Milliseconds()
	e.Timezone = NormalizeTimezone(e.Country, e.Timezone)

	// Two more fresh connections: a sticky proxy keeps its IP, a rotating one does not.
	fresh := clientVia(up, false)
	for i := 0; i < 2; i++ {
		ip, err := getText(ctx, fresh, ipifyURL)
		if err == nil && ip != "" && ip != e.IP {
			e.Rotating = true
			break
		}
	}
	return e, nil
}

func getText(ctx context.Context, c *http.Client, u string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", u, resp.Status)
	}
	return strings.TrimSpace(string(b)), nil
}

func geoIPInfo(ctx context.Context, c *http.Client) (Exit, error) {
	body, err := getText(ctx, c, ipinfoURL)
	if err != nil {
		return Exit{}, err
	}
	var r struct{ IP, Country, City, Loc, Timezone string }
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		return Exit{}, err
	}
	if r.IP == "" {
		return Exit{}, errors.New("ipinfo: empty answer")
	}
	e := Exit{IP: r.IP, Country: r.Country, City: r.City, Timezone: r.Timezone, Source: "ipinfo"}
	if lat, lon, ok := strings.Cut(r.Loc, ","); ok {
		if a, err := strconv.ParseFloat(lat, 64); err == nil {
			if b, err := strconv.ParseFloat(lon, 64); err == nil {
				e.Lat, e.Lon = &a, &b
			}
		}
	}
	return e, nil
}

func geoIPWho(ctx context.Context, c *http.Client) (Exit, error) {
	body, err := getText(ctx, c, ipwhoURL)
	if err != nil {
		return Exit{}, err
	}
	var r struct {
		IP          string  `json:"ip"`
		Success     bool    `json:"success"`
		CountryCode string  `json:"country_code"`
		City        string  `json:"city"`
		Latitude    float64 `json:"latitude"`
		Longitude   float64 `json:"longitude"`
		Timezone    struct {
			ID string `json:"id"`
		} `json:"timezone"`
	}
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		return Exit{}, err
	}
	if !r.Success || r.IP == "" {
		return Exit{}, errors.New("ipwho.is: lookup failed")
	}
	return Exit{IP: r.IP, Country: r.CountryCode, City: r.City, Timezone: r.Timezone.ID,
		Lat: &r.Latitude, Lon: &r.Longitude, Source: "ipwho.is"}, nil
}
