// SPDX-License-Identifier: AGPL-3.0-or-later

package fingerprint

import (
	"encoding/json"
	"testing"
)

func TestCompare(t *testing.T) {
	base := Sample{Canvas: "a", WebGLVendor: "v", WebGLRenderer: "r", UA: "u", Platform: "p", HWC: 8, Mem: 8, Screen: "1710x1112", TZ: "America/New_York", Langs: "en-US,en"}
	same := base
	if v, d := Compare(base, same, "America/New_York", "en-US,en", false); v != "match" || d != nil {
		t.Fatalf("same = %q %v", v, d)
	}

	got := base
	got.Canvas = "b"
	if v, d := Compare(base, got, "America/New_York", "en-US,en", false); v != "drift" || len(d) != 1 || d[0] != "canvas" {
		t.Fatalf("canvas drift = %q %v", v, d)
	}

	got = base
	got.TZ = "Asia/Ho_Chi_Minh"
	if v, d := Compare(base, got, "America/New_York", "en-US,en", false); v != "drift" || len(d) != 1 || d[0] != "tz" {
		t.Fatalf("tz drift = %q %v", v, d)
	}

	// 8 and 8.0 are equal floats.
	got = base
	got.Mem = 8.0
	if v, _ := Compare(base, got, "America/New_York", "en-US,en", false); v != "match" {
		t.Fatalf("mem 8 vs 8.0 = %q", v)
	}

	// The browser may report a deprecated zone link; it must not count as drift.
	got = base
	got.TZ = "Asia/Saigon"
	if v, d := Compare(base, got, "Asia/Ho_Chi_Minh", "en-US,en", false); v != "match" || d != nil {
		t.Fatalf("Saigon vs Ho_Chi_Minh = %q %v", v, d)
	}
}

func TestLangsMatch(t *testing.T) {
	cases := []struct {
		got, want   string
		primaryOnly bool
		ok          bool
	}{
		{"en-US,en", "en-US,en", false, true},
		{"en-US", "en-US,en", false, false},
		{"vi", "vi-VN,vi,en-US,en", true, true},
		{"zh-TW", "zh-TW,zh,en", true, true},
		{"en-US", "vi-VN,vi", true, false},
		{"", "en-US", true, false},
	}
	for _, c := range cases {
		if got := LangsMatch(c.got, c.want, c.primaryOnly); got != c.ok {
			t.Errorf("LangsMatch(%q, %q, %v) = %v", c.got, c.want, c.primaryOnly, got)
		}
	}
	base := Sample{Langs: "vi"}
	if v, d := Compare(base, base, "", "vi-VN,vi,en", true); v != "match" {
		t.Fatalf("primary-only compare = %q %v", v, d)
	}
}

func TestSampleReadsConsistencySignals(t *testing.T) {
	var s Sample
	raw := `{"canvas":"a","ua":"Chrome/155.0.0.0","webdriver":true,"chromeH":87,"geo":"denied","ice":["abc.local","203.0.113.7"]}`
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatal(err)
	}
	if !s.WebDriver || s.ChromeH != 87 || s.Geo != "denied" || len(s.ICE) != 2 {
		t.Fatalf("sample = %+v", s)
	}
}

func TestCompareOrRenew(t *testing.T) {
	const ua155 = "Mozilla/5.0 (Macintosh) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36"
	const ua156 = "Mozilla/5.0 (Macintosh) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/156.0.0.0 Safari/537.36"
	old := Sample{Canvas: "4b92cc9c0a0f033b", UA: ua155, Platform: "MacIntel", HWC: 8, Mem: 16, Screen: "1712x1112", TZ: "Asia/Saigon", Langs: "en-US"}
	raw, _ := json.Marshal(old)

	// Nothing stored yet: this measurement becomes the baseline.
	if v, d := CompareOrRenew("", old, "Asia/Saigon", "en-US", true); v != "baseline" || d != nil {
		t.Fatalf("empty baseline = %q %v", v, d)
	}
	// An unreadable baseline starts over instead of comparing with nothing.
	if v, _ := CompareOrRenew("{not json", old, "Asia/Saigon", "en-US", true); v != "baseline" {
		t.Fatalf("corrupt baseline = %q", v)
	}
	// Same build, nothing moved: a plain match.
	if v, d := CompareOrRenew(string(raw), old, "Asia/Saigon", "en-US", true); v != "match" || d != nil {
		t.Fatalf("same build = %q %v", v, d)
	}

	// The engine moved to a new Chromium: the user agent and the canvas hash
	// both change, which is the browser's doing, not the profile's. That
	// replaces the baseline instead of reporting drift on every launch.
	updated := old
	updated.UA = ua156
	updated.Canvas = "07464365c6ce7358"
	if v, d := CompareOrRenew(string(raw), updated, "Asia/Saigon", "en-US", true); v != "renewed" || d != nil {
		t.Fatalf("browser update = %q %v", v, d)
	}
	if !Renewed(old, updated) {
		t.Fatal("a new Chromium major is not a renewal")
	}
	// A patch release keeps the major in the user agent: same build, so the
	// values stay comparable and a real change still shows as drift.
	patch := old
	patch.Canvas = "different"
	if Renewed(old, patch) {
		t.Fatal("a patch release is not a renewal")
	}
	if v, d := CompareOrRenew(string(raw), patch, "Asia/Saigon", "en-US", true); v != "drift" || len(d) != 1 || d[0] != "canvas" {
		t.Fatalf("canvas drift on the same build = %q %v", v, d)
	}

	// A baseline that names no browser cannot describe one either.
	bare := Sample{Canvas: "x"}
	if v, _ := CompareOrRenew(mustJSON(t, bare), old, "Asia/Saigon", "en-US", true); v != "renewed" {
		t.Fatalf("baseline without a user agent = %q", v)
	}
}

func mustJSON(t *testing.T, s Sample) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
