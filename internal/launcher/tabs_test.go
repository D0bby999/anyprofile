// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
)

func TestWebTabs(t *testing.T) {
	ts := []targetInfo{
		{Type: "page", URL: "https://www.tiktok.com/tiktokstudio"},
		{Type: "page", URL: "chrome://newtab/"},
		{Type: "page", URL: "about:blank"},
		{Type: "page", URL: "https://fp.anyprofile.invalid/"},
		{Type: "page", URL: "http://127.0.0.1:8080/a"},
		{Type: "page", URL: "file:///etc/passwd"},
	}
	got := webTabs(ts)
	want := []string{"https://www.tiktok.com/tiktokstudio", "http://127.0.0.1:8080/a"}
	if !slices.Equal(got, want) {
		t.Fatalf("webTabs = %v", got)
	}
	many := make([]targetInfo, 30)
	for i := range many {
		many[i] = targetInfo{Type: "page", URL: "https://a.example/" + strconv.Itoa(i)}
	}
	if n := len(webTabs(many)); n != maxTabs {
		t.Fatalf("kept %d tabs, want %d", n, maxTabs)
	}
}

func TestTabsFileRoundTrip(t *testing.T) {
	udd := t.TempDir()
	if got := readTabs(udd); got != nil {
		t.Fatalf("no file read as %v", got)
	}
	tabs := []string{"https://a.example/", "https://b.example/x?y=1"}
	if err := writeTabs(udd, tabs); err != nil {
		t.Fatal(err)
	}
	if got := readTabs(udd); !slices.Equal(got, tabs) {
		t.Fatalf("readTabs = %v", got)
	}
	fi, err := os.Stat(filepath.Join(udd, tabsFile))
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("tabs file mode = %v, %v", fi.Mode().Perm(), err)
	}
	// Anything not http(s) in the file is dropped on the way back in.
	os.WriteFile(filepath.Join(udd, tabsFile), []byte(`["javascript:alert(1)","https://ok.example/"]`), 0o600)
	if got := readTabs(udd); !slices.Equal(got, []string{"https://ok.example/"}) {
		t.Fatalf("readTabs kept %v", got)
	}
	os.WriteFile(filepath.Join(udd, tabsFile), []byte(`not json`), 0o600)
	if got := readTabs(udd); got != nil {
		t.Fatalf("bad file read as %v", got)
	}
}
