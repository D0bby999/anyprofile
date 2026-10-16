// SPDX-License-Identifier: AGPL-3.0-or-later

package fingerprint

import "testing"

func cleanSample() Sample {
	return Sample{UA: "Mozilla/5.0 (Macintosh) Chrome/155.0.0.0 Safari/537.36", Platform: "MacIntel", HWC: 8,
		TZ: "Asia/Taipei", Langs: "zh-TW", ChromeH: 87, Geo: "denied", ICE: []string{"1f2e.local"}}
}

func apostateExpect() Expect {
	return Expect{Engine: "apostate", Timezone: "Asia/Taipei", Languages: "zh-TW,zh,en", ChromiumMajor: 155,
		Proxied: true, HWC: 8, PrimaryLangOnly: true}
}

func TestChecksAllPass(t *testing.T) {
	got := Checks(cleanSample(), apostateExpect())
	want := []string{"webdriver", "webrtc_ip", "tz_profile", "langs", "ua_version", "platform", "chrome_height", "geolocation", "hwc_pin"}
	if len(got) != len(want) {
		t.Fatalf("checks = %+v", got)
	}
	for i, c := range got {
		if c.ID != want[i] || !c.OK || c.Got != "" {
			t.Errorf("check %d = %+v, want %s ok", i, c, want[i])
		}
	}
}

func TestChecksEachFailure(t *testing.T) {
	cases := map[string]struct {
		level string
		bend  func(*Sample)
	}{
		"webdriver":     {"danger", func(s *Sample) { s.WebDriver = true }},
		"webrtc_ip":     {"danger", func(s *Sample) { s.ICE = []string{"x.local", "203.0.113.7"} }},
		"tz_profile":    {"danger", func(s *Sample) { s.TZ = "Asia/Tokyo" }},
		"langs":         {"warn", func(s *Sample) { s.Langs = "en-US" }},
		"ua_version":    {"danger", func(s *Sample) { s.UA = "Chrome/150.0.0.0" }},
		"platform":      {"danger", func(s *Sample) { s.Platform = "Win32" }},
		"chrome_height": {"warn", func(s *Sample) { s.ChromeH = 143 }},
		"geolocation":   {"warn", func(s *Sample) { s.Geo = "granted" }},
		"hwc_pin":       {"warn", func(s *Sample) { s.HWC = 4 }},
	}
	for id, c := range cases {
		s := cleanSample()
		c.bend(&s)
		for _, ch := range Checks(s, apostateExpect()) {
			if ch.ID == id && (ch.OK || ch.Level != c.level || ch.Got == "") {
				t.Errorf("%s = %+v, want failed at %s", id, ch, c.level)
			}
			if ch.ID != id && !ch.OK {
				t.Errorf("bending %s also failed %s", id, ch.ID)
			}
		}
	}
}

func TestChecksApplyByEngineAndProxy(t *testing.T) {
	e := apostateExpect()
	e.Engine, e.Proxied, e.PrimaryLangOnly = "fury", false, false
	s := cleanSample()
	s.Langs, s.ChromeH = "zh-TW,zh,en", -1
	ids := map[string]bool{}
	for _, c := range Checks(s, e) {
		ids[c.ID] = true
	}
	for _, absent := range []string{"geolocation", "hwc_pin", "webrtc_ip", "chrome_height"} {
		if ids[absent] {
			t.Errorf("%s applied to a Fury profile without proxy and without chrome height", absent)
		}
	}
	if len(ids) != 5 {
		t.Errorf("checks = %v", ids)
	}
	if MajorVersion("155.0.8059.31") != 155 {
		t.Error("MajorVersion")
	}
}

func TestPlatformFollowsProfileOS(t *testing.T) {
	win := cleanSample()
	win.Platform = "Win32"
	for _, c := range []struct {
		os   string
		s    Sample
		pass bool
	}{{"macos", cleanSample(), true}, {"", cleanSample(), true}, {"windows", win, true}, {"windows", cleanSample(), false}, {"macos", win, false}} {
		e := apostateExpect()
		e.OS = c.os
		for _, ch := range Checks(c.s, e) {
			if ch.ID == "platform" && ch.OK != c.pass {
				t.Errorf("os %q platform %q: ok = %v, want %v", c.os, c.s.Platform, ch.OK, c.pass)
			}
		}
	}
}
