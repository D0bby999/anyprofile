// SPDX-License-Identifier: AGPL-3.0-or-later

package fingerprint

import "testing"

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
