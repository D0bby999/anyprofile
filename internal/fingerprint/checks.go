// SPDX-License-Identifier: AGPL-3.0-or-later

package fingerprint

import (
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/D0bby999/anyprofile/internal/netproxy"
)

// Check is one consistency condition a detection site could test.
type Check struct {
	ID    string `json:"id"`
	OK    bool   `json:"ok"`
	Level string `json:"level"` // level when not OK: "warn" or "danger"
	Got   string `json:"got,omitempty"`
	Want  string `json:"want,omitempty"`
}

// Expect is what the profile and its engine promise.
type Expect struct {
	Engine, Timezone, Languages string
	ChromiumMajor               int
	Proxied                     bool
	HWC                         int  // 0: not pinned
	PrimaryLangOnly             bool // the hidden tab sees only the primary language
}

// maxChromeHeight is the browser chrome above a page without an info bar;
// Chrome on macOS measures 87.
const maxChromeHeight = 120

var chromeMajor = regexp.MustCompile(`Chrome/(\d+)\.`)

// Checks runs the consistency conditions that apply, in a fixed order.
func Checks(s Sample, e Expect) []Check {
	out := []Check{}
	add := func(id, level string, ok bool, got, want string) {
		c := Check{ID: id, OK: ok, Level: level}
		if !ok {
			c.Got, c.Want = got, want
		}
		out = append(out, c)
	}
	add("webdriver", "danger", !s.WebDriver, "true", "false")
	if e.Proxied {
		var leaked []string
		for _, a := range s.ICE {
			if a != "" && !strings.HasSuffix(a, ".local") && net.ParseIP(a) != nil {
				leaked = append(leaked, a)
			}
		}
		add("webrtc_ip", "danger", len(leaked) == 0, strings.Join(leaked, ", "), "none")
	}
	add("tz_profile", "danger", netproxy.CanonicalZone(s.TZ) == netproxy.CanonicalZone(e.Timezone), s.TZ, e.Timezone)
	add("langs", "warn", LangsMatch(s.Langs, e.Languages, e.PrimaryLangOnly), s.Langs, e.Languages)
	got := ""
	if m := chromeMajor.FindStringSubmatch(s.UA); m != nil {
		got = m[1]
	}
	add("ua_version", "danger", got == strconv.Itoa(e.ChromiumMajor), got, strconv.Itoa(e.ChromiumMajor))
	add("platform", "danger", s.Platform == "MacIntel", s.Platform, "MacIntel")
	if s.ChromeH >= 0 {
		add("chrome_height", "warn", s.ChromeH <= maxChromeHeight, strconv.Itoa(s.ChromeH), "≤ "+strconv.Itoa(maxChromeHeight))
	}
	if e.Engine == "apostate" {
		add("geolocation", "warn", s.Geo == "denied" || s.Geo == "prompt", s.Geo, "denied")
		if e.HWC > 0 {
			add("hwc_pin", "warn", s.HWC == e.HWC, strconv.Itoa(s.HWC), strconv.Itoa(e.HWC))
		}
	}
	return out
}

// MajorVersion is the leading number of a version string ("155.0.8059.31" -> 155).
func MajorVersion(v string) int {
	n, _ := strconv.Atoi(strings.SplitN(v, ".", 2)[0])
	return n
}
