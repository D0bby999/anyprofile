// SPDX-License-Identifier: AGPL-3.0-or-later

// Package bookmarks reads the bookmark HTML file every browser exports and
// adds it to a Chromium profile's Bookmarks file.
package bookmarks

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	MaxFileBytes = 5 << 20
	maxItems     = 10000
)

// Node is a folder (Children set) or a link (URL set).
type Node struct {
	Name     string
	URL      string
	Children []*Node
}

var tokenRe = regexp.MustCompile(`(?is)<h3[^>]*>(.*?)</h3>|<a\s[^>]*?href\s*=\s*"([^"]*)"[^>]*>(.*?)</a>|<dl[^>]*>|</dl>`)

// Parse reads the Netscape bookmark format. Only http and https links are
// kept; javascript: and file: links are dropped. It returns the root folder
// and the number of links.
func Parse(b []byte) (*Node, int, error) {
	root := &Node{}
	stack := []*Node{root}
	var pending *Node // a folder named by <H3>, opened by the next <DL>
	n := 0
	for _, m := range tokenRe.FindAllSubmatch(b, -1) {
		tok := strings.ToLower(string(m[0][:min(4, len(m[0]))]))
		cur := stack[len(stack)-1]
		switch {
		case strings.HasPrefix(tok, "<h3"):
			pending = &Node{Name: clean(m[1]), Children: []*Node{}}
			cur.Children = append(cur.Children, pending)
		case strings.HasPrefix(tok, "<a"):
			u := html.UnescapeString(string(m[2]))
			if p, err := url.Parse(u); err != nil || (p.Scheme != "http" && p.Scheme != "https") {
				continue
			}
			if n++; n > maxItems {
				return nil, 0, fmt.Errorf("more than %d bookmarks", maxItems)
			}
			name := clean(m[3])
			if name == "" {
				name = u
			}
			cur.Children = append(cur.Children, &Node{Name: name, URL: u})
		case strings.HasPrefix(tok, "</dl"):
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		default: // <dl
			if pending != nil {
				stack = append(stack, pending)
				pending = nil
			} else if len(stack) > 1 || len(root.Children) > 0 {
				stack = append(stack, cur) // a bare <DL> keeps the level balanced
			}
		}
	}
	if n == 0 {
		return nil, 0, errors.New("no http or https bookmarks in the file")
	}
	return root, n, nil
}

var tagRe = regexp.MustCompile(`<[^>]*>`)

func clean(b []byte) string {
	s := strings.TrimSpace(html.UnescapeString(tagRe.ReplaceAllString(string(b), "")))
	if r := []rune(s); len(r) > 500 {
		s = string(r[:500])
	}
	return s
}

// AddToProfile puts the bookmarks in a folder named folder on the bookmarks
// bar of <udd>/Default/Bookmarks, keeping what is there. The profile must be
// closed: Chromium rewrites the file on exit.
func AddToProfile(udd, folder string, root *Node) error {
	path := filepath.Join(udd, "Default", "Bookmarks")
	doc := map[string]any{}
	switch b, err := os.ReadFile(path); {
	case errors.Is(err, os.ErrNotExist):
		doc = emptyDoc()
	case err != nil:
		return err
	default:
		if err := json.Unmarshal(b, &doc); err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
	}
	roots, _ := doc["roots"].(map[string]any)
	bar, _ := roots["bookmark_bar"].(map[string]any)
	if bar == nil {
		return errors.New("the profile's Bookmarks file has no bookmarks bar")
	}
	next := maxID(doc) + 1
	now := strconv.FormatInt(webkitNow(), 10)
	kids, _ := bar["children"].([]any)
	bar["children"] = append(kids, toJSON(&Node{Name: folder, Children: root.Children}, &next, now))
	// The checksum no longer matches; Chromium accepts a file without one.
	delete(doc, "checksum")
	out, err := json.MarshalIndent(doc, "", "   ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".anyprofile-tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func toJSON(n *Node, next *int, now string) map[string]any {
	id := strconv.Itoa(*next)
	*next++
	m := map[string]any{"id": id, "guid": uuid4(), "name": n.Name, "date_added": now}
	if n.URL != "" {
		m["type"], m["url"] = "url", n.URL
		return m
	}
	kids := make([]any, 0, len(n.Children))
	for _, c := range n.Children {
		kids = append(kids, toJSON(c, next, now))
	}
	m["type"], m["children"], m["date_modified"] = "folder", kids, now
	return m
}

func emptyDoc() map[string]any {
	folder := func(id, name, guid string) map[string]any {
		return map[string]any{"id": id, "guid": guid, "name": name, "type": "folder", "children": []any{},
			"date_added": "0", "date_modified": "0"}
	}
	return map[string]any{"version": 1, "roots": map[string]any{
		"bookmark_bar": folder("1", "Bookmarks bar", "0bc5d13f-2cba-5d74-951f-3f233fe6c908"),
		"other":        folder("2", "Other bookmarks", "82b081ec-3dd3-529c-8475-ab6c344590dd"),
		"synced":       folder("3", "Mobile bookmarks", "4cf2e351-0e85-532b-bb37-df045d8f8d0f"),
	}}
}

func maxID(v any) int {
	best := 0
	switch x := v.(type) {
	case map[string]any:
		if s, ok := x["id"].(string); ok {
			if n, err := strconv.Atoi(s); err == nil && n > best {
				best = n
			}
		}
		for _, c := range x {
			best = max(best, maxID(c))
		}
	case []any:
		for _, c := range x {
			best = max(best, maxID(c))
		}
	}
	return best
}

// webkitNow is microseconds since 1601-01-01, Chromium's time base.
func webkitNow() int64 {
	return time.Now().UnixMicro() + 11644473600*1_000_000
}

func uuid4() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
