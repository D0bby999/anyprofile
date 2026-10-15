// SPDX-License-Identifier: AGPL-3.0-or-later

package site

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestCatalog(t *testing.T) {
	entries := Builtin()
	if len(entries) == 0 {
		t.Fatal("catalog is empty")
	}
	idRe := regexp.MustCompile(`^[a-z0-9-]+$`)
	colorRe := regexp.MustCompile(`^#[0-9A-F]{6}$`)
	groups := map[string]bool{"video": true, "social": true, "messaging": true, "commerce": true,
		"ads": true, "freelance": true, "google": true, "payments": true, "other": true}
	seenID := map[string]bool{}
	seenDomain := map[string]string{}
	for _, s := range entries {
		if !idRe.MatchString(s.ID) {
			t.Errorf("%q: bad id", s.ID)
		}
		if seenID[s.ID] {
			t.Errorf("%q: duplicate id", s.ID)
		}
		seenID[s.ID] = true
		if strings.HasPrefix(s.ID, "custom-") {
			t.Errorf("%q: built-in ids must not use the custom- prefix", s.ID)
		}
		if s.Name == "" {
			t.Errorf("%q: empty name", s.ID)
		}
		if !groups[s.Group] {
			t.Errorf("%q: bad group %q", s.ID, s.Group)
		}
		if !colorRe.MatchString(s.Color) {
			t.Errorf("%q: bad color %q", s.ID, s.Color)
		}
		// A country's start page must be https, on one of the platform's own
		// domains (so the platform is recognized from it), keyed by ISO code.
		for cc, u := range s.StartURLs {
			if !regexp.MustCompile(`^[A-Z]{2}$`).MatchString(cc) || cc == "UK" {
				t.Errorf("%q: bad country %q", s.ID, cc)
			}
			host := strings.TrimPrefix(u, "https://")
			if host == u || !slices.Contains(s.Domains, strings.SplitN(host, "/", 2)[0]) {
				t.Errorf("%q: start page for %s %q is not https on its domains", s.ID, cc, u)
			}
		}
		if s.StartURL != "" && !strings.HasPrefix(s.StartURL, "https://") {
			t.Errorf("%q: start url must be https: %q", s.ID, s.StartURL)
		}
		for _, d := range s.Domains {
			if d != strings.ToLower(d) {
				t.Errorf("%q: domain %q must be lowercase", s.ID, d)
			}
			if prev, ok := seenDomain[d]; ok {
				t.Errorf("domain %q in both %q and %q", d, prev, s.ID)
			}
			seenDomain[d] = s.ID
		}
	}
	for _, want := range []string{"other", "tiktok", "youtube"} {
		if !seenID[want] {
			t.Errorf("catalog must contain %q (the migration needs it)", want)
		}
	}
}
