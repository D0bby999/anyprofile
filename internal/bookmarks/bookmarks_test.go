// SPDX-License-Identifier: AGPL-3.0-or-later

package bookmarks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const export = `<!DOCTYPE NETSCAPE-Bookmark-file-1>
<META HTTP-EQUIV="Content-Type" CONTENT="text/html; charset=UTF-8">
<TITLE>Bookmarks</TITLE>
<H1>Bookmarks</H1>
<DL><p>
    <DT><H3 ADD_DATE="1" PERSONAL_TOOLBAR_FOLDER="true">Bookmarks bar</H3>
    <DL><p>
        <DT><A HREF="https://www.tiktok.com/tiktokstudio" ADD_DATE="1">TikTok Studio</A>
        <DT><H3>Shop &amp; ads</H3>
        <DL><p>
            <DT><A HREF="https://www.etsy.com/your/shops/me">Etsy</A>
            <DT><A HREF="javascript:alert(1)">Bookmarklet</A>
        </DL><p>
    </DL><p>
    <DT><A HREF="https://mail.google.com/">Gmail</A>
</DL><p>
`

func TestParse(t *testing.T) {
	root, n, err := Parse([]byte(export))
	if err != nil || n != 3 {
		t.Fatalf("n = %d, %v", n, err)
	}
	bar := root.Children[0]
	if bar.Name != "Bookmarks bar" || len(bar.Children) != 2 || bar.Children[0].URL != "https://www.tiktok.com/tiktokstudio" {
		t.Fatalf("bar = %+v", bar)
	}
	shop := bar.Children[1]
	if shop.Name != "Shop & ads" || len(shop.Children) != 1 || shop.Children[0].Name != "Etsy" {
		t.Fatalf("shop = %+v (the bookmarklet must be dropped)", shop)
	}
	if len(root.Children) != 2 || root.Children[1].Name != "Gmail" {
		t.Fatalf("root = %+v", root.Children)
	}
	if _, _, err := Parse([]byte(`<DL><DT><A HREF="file:///etc/passwd">x</A></DL>`)); err == nil {
		t.Fatal("a file with no web links must fail")
	}
}

func TestAddToProfileKeepsWhatIsThere(t *testing.T) {
	udd := t.TempDir()
	root, _, _ := Parse([]byte(export))
	if err := AddToProfile(udd, "Imported", root); err != nil {
		t.Fatal(err)
	}
	root2, _, _ := Parse([]byte(`<DL><DT><A HREF="https://example.com/">Example</A></DL>`))
	if err := AddToProfile(udd, "Imported 2", root2); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(udd, "Default", "Bookmarks"))
	var doc struct {
		Checksum string `json:"checksum"`
		Roots    struct {
			Bar struct {
				Children []struct {
					ID, Name, Type string
					Children       []json.RawMessage
				} `json:"children"`
			} `json:"bookmark_bar"`
		} `json:"roots"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	kids := doc.Roots.Bar.Children
	if len(kids) != 2 || kids[0].Name != "Imported" || kids[1].Name != "Imported 2" || kids[0].Type != "folder" {
		t.Fatalf("bar = %+v", kids)
	}
	if kids[0].ID == kids[1].ID || kids[0].ID == "1" {
		t.Fatalf("ids clash: %s %s", kids[0].ID, kids[1].ID)
	}
	if doc.Checksum != "" {
		t.Fatal("a stale checksum was kept")
	}
	if fi, _ := os.Stat(filepath.Join(udd, "Default", "Bookmarks")); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", fi.Mode().Perm())
	}
}
