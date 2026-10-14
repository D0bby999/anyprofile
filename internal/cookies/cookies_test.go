// SPDX-License-Identifier: AGPL-3.0-or-later

package cookies

import (
	"os"
	"strings"
	"testing"
)

func parseFile(t *testing.T, name string) ([]Cookie, int) {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	cs, skipped, err := Parse(b)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return cs, skipped
}

func TestParseCookieEditor(t *testing.T) {
	cs, skipped := parseFile(t, "cookie-editor.json")
	if len(cs) != 2 || skipped != 1 {
		t.Fatalf("got %d cookies, %d skipped", len(cs), skipped)
	}
	if c := cs[0]; c.Name != "sid" || !c.HTTPOnly || !c.Secure || c.SameSite != "None" || c.Expires != 4102444800 {
		t.Fatalf("sid = %+v", c)
	}
	if c := cs[1]; c.Expires != 0 || c.SameSite != "Lax" {
		t.Fatalf("session cookie = %+v", c)
	}
}

func TestParseEditThisCookie(t *testing.T) {
	cs, skipped := parseFile(t, "editthiscookie.json")
	if len(cs) != 1 || skipped != 1 || cs[0].SameSite != "Strict" {
		t.Fatalf("got %+v, %d skipped", cs, skipped)
	}
}

func TestParseNetscape(t *testing.T) {
	cs, _ := parseFile(t, "cookies.txt")
	if len(cs) != 2 {
		t.Fatalf("got %+v", cs)
	}
	if c := cs[0]; c.Name != "auth" || !c.HTTPOnly || !c.Secure || c.Domain != ".example.com" {
		t.Fatalf("auth = %+v", c)
	}
	if c := cs[1]; c.HTTPOnly || c.Expires != 0 {
		t.Fatalf("lang = %+v", c)
	}
}

func TestExportRoundTrip(t *testing.T) {
	cs, _ := parseFile(t, "cookie-editor.json")
	b, err := Export(cs)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"hostOnly": true`) || !strings.Contains(string(b), `"sameSite": "no_restriction"`) {
		t.Fatalf("export = %s", b)
	}
	again, skipped, err := Parse(b)
	if err != nil || skipped != 0 || len(again) != len(cs) {
		t.Fatalf("re-parse = %d %d %v", len(again), skipped, err)
	}
	for i := range cs {
		if again[i] != cs[i] {
			t.Errorf("cookie %d: %+v != %+v", i, again[i], cs[i])
		}
	}
}

func TestParseRejects(t *testing.T) {
	for name, data := range map[string]string{
		"broken json": "[{\"name\": \"a\",\n \"domain\": }]",
		"no netscape": "just some text\nnothing here\n",
		"too many":    "[" + strings.TrimSuffix(strings.Repeat(`{"name":"a","value":"b","domain":".x.com"},`, MaxCookies+1), ",") + "]",
	} {
		if _, _, err := Parse([]byte(data)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, _, err := Parse([]byte("[{\"name\": \"a\",\n \"domain\": }]")); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Errorf("JSON error should name the line: %v", err)
	}
	long := `[{"name":"a","domain":".x.com","value":"` + strings.Repeat("v", 5000) + `"}]`
	if cs, skipped, err := Parse([]byte(long)); err != nil || len(cs) != 0 || skipped != 1 {
		t.Errorf("oversized value: %d %d %v", len(cs), skipped, err)
	}
}

func TestTopDomains(t *testing.T) {
	cs := []Cookie{{Domain: ".tiktok.com"}, {Domain: "www.tiktok.com"}, {Domain: ".google.com"}}
	if got := TopDomains(cs, 5); len(got) != 2 || got[0] != "tiktok.com" || got[1] != "google.com" {
		t.Fatalf("top = %v", got)
	}
}
