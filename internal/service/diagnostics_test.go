// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	home, _ := os.UserHomeDir()
	in := `proxy=http://alice-sticky__cr.us@gw.example.net:10000 exit 198.51.100.23 ip6 2001:db8:83f9:c010:b389:dab8:1abe:7dc6 ` +
		`mail me@example.com password=hunter2 token: abc relay 127.0.0.1:41000 dir ` + filepath.Join(home, "Library")
	got := Redact(in)
	for _, leak := range []string{"alice-sticky", "198.51.100.23", "2001:db8", "me@example.com", "hunter2", "abc", home} {
		if strings.Contains(got, leak) {
			t.Errorf("leaked %q in %q", leak, got)
		}
	}
	for _, keep := range []string{"gw.example.net:10000", "127.0.0.1:41000", "~"} {
		if !strings.Contains(got, keep) {
			t.Errorf("lost %q in %q", keep, got)
		}
	}
}

func TestDiagnosticsNamesNoProfileOrProxy(t *testing.T) {
	s, _ := newService(t)
	os.WriteFile(filepath.Join(s.Dirs.Logs(), "anyprofile.log"), []byte("level=WARN msg=x proxy=http://bob@1.2.3.4:8080\n"), 0o600)
	ctx := context.Background()
	if _, err := s.ImportProxies(ctx, "1.2.3.4:8080:bob:pw", ""); err != nil {
		t.Fatal(err)
	}
	d := s.Diagnostics(ctx)
	for _, want := range []string{"AnyProfile ", "OS: ", "Proxies: 1", "msg=x"} {
		if !strings.Contains(d, want) {
			t.Errorf("missing %q:\n%s", want, d)
		}
	}
	for _, leak := range []string{"bob", "1.2.3.4", "pw"} {
		if strings.Contains(d, leak) {
			t.Errorf("leaked %q:\n%s", leak, d)
		}
	}
}
