// SPDX-License-Identifier: AGPL-3.0-or-later

package netproxy

import (
	"strings"
	"time"
	_ "time/tzdata" // validate zone names without depending on the host's zoneinfo
)

// legacyZones maps deprecated IANA links to their canonical names. Fingerprint
// checkers compare names literally, so "Asia/Saigon" counts as a mismatch.
var legacyZones = map[string]string{
	"Asia/Saigon":          "Asia/Ho_Chi_Minh",
	"Asia/Calcutta":        "Asia/Kolkata",
	"Asia/Katmandu":        "Asia/Kathmandu",
	"Asia/Rangoon":         "Asia/Yangon",
	"Asia/Ujung_Pandang":   "Asia/Makassar",
	"Europe/Kiev":          "Europe/Kyiv",
	"America/Buenos_Aires": "America/Argentina/Buenos_Aires",
	"US/Eastern":           "America/New_York",
	"US/Central":           "America/Chicago",
	"US/Mountain":          "America/Denver",
	"US/Pacific":           "America/Los_Angeles",
}

// singleZone holds countries with one zone in practice. GeoIP providers
// sometimes return a neighbour's zone (ipinfo gives Asia/Bangkok for some
// Vietnamese ranges); for these countries the country code wins.
var singleZone = map[string]string{
	"VN": "Asia/Ho_Chi_Minh", "TH": "Asia/Bangkok", "PH": "Asia/Manila", "SG": "Asia/Singapore",
	"MY": "Asia/Kuala_Lumpur", "JP": "Asia/Tokyo", "KR": "Asia/Seoul", "TW": "Asia/Taipei",
	"HK": "Asia/Hong_Kong", "IN": "Asia/Kolkata", "KH": "Asia/Phnom_Penh", "LA": "Asia/Vientiane",
	"GB": "Europe/London", "IE": "Europe/Dublin", "FR": "Europe/Paris", "DE": "Europe/Berlin",
	"IT": "Europe/Rome", "NL": "Europe/Amsterdam", "BE": "Europe/Brussels", "PL": "Europe/Warsaw",
	"SE": "Europe/Stockholm", "NO": "Europe/Oslo", "DK": "Europe/Copenhagen", "FI": "Europe/Helsinki",
	"CH": "Europe/Zurich", "AT": "Europe/Vienna", "CZ": "Europe/Prague", "UA": "Europe/Kyiv",
	"TR": "Europe/Istanbul", "AE": "Asia/Dubai", "IL": "Asia/Jerusalem", "SA": "Asia/Riyadh",
	"NZ": "Pacific/Auckland", "ZA": "Africa/Johannesburg", "NG": "Africa/Lagos", "EG": "Africa/Cairo",
}

// NormalizeTimezone returns a canonical, loadable IANA zone for a GeoIP answer,
// or "" when nothing usable is known.
func NormalizeTimezone(country, tz string) string {
	if z, ok := singleZone[strings.ToUpper(country)]; ok {
		return z
	}
	if tz == "" {
		return ""
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return ""
	}
	return CanonicalZone(tz)
}

// CanonicalZone maps a deprecated IANA link (for example "Asia/Saigon") to its
// canonical name, so literal comparisons match. Unknown names pass through.
func CanonicalZone(tz string) string {
	if z, ok := legacyZones[tz]; ok {
		return z
	}
	return tz
}
