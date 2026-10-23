// SPDX-License-Identifier: AGPL-3.0-or-later

package control

import (
	"encoding/xml"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Rect is a node's bounds on screen, in pixels.
type Rect struct{ X1, Y1, X2, Y2 int }

func (r Rect) Center() (int, int) { return (r.X1 + r.X2) / 2, (r.Y1 + r.Y2) / 2 }
func (r Rect) Empty() bool        { return r.X2 <= r.X1 || r.Y2 <= r.Y1 }

type uiNode struct {
	Text       string   `xml:"text,attr"`
	ResourceID string   `xml:"resource-id,attr"`
	Class      string   `xml:"class,attr"`
	Package    string   `xml:"package,attr"`
	Desc       string   `xml:"content-desc,attr"`
	Checkable  bool     `xml:"checkable,attr"`
	Checked    bool     `xml:"checked,attr"`
	Clickable  bool     `xml:"clickable,attr"`
	LongClick  bool     `xml:"long-clickable,attr"`
	Enabled    bool     `xml:"enabled,attr"`
	Focused    bool     `xml:"focused,attr"`
	Scrollable bool     `xml:"scrollable,attr"`
	Password   bool     `xml:"password,attr"`
	Selected   bool     `xml:"selected,attr"`
	Bounds     string   `xml:"bounds,attr"`
	Nodes      []uiNode `xml:"node"`
}

type uiHierarchy struct {
	Nodes []uiNode `xml:"node"`
}

var boundsRe = regexp.MustCompile(`^\[(-?\d+),(-?\d+)\]\[(-?\d+),(-?\d+)\]$`)

func parseBounds(s string) (Rect, bool) {
	m := boundsRe.FindStringSubmatch(s)
	if m == nil {
		return Rect{}, false
	}
	var v [4]int
	for i := range v {
		v[i], _ = strconv.Atoi(m[i+1])
	}
	return Rect{v[0], v[1], v[2], v[3]}, true
}

// shortClass is "Button" for "android.widget.Button".
func shortClass(c string) string {
	if i := strings.LastIndexByte(c, '.'); i >= 0 {
		return c[i+1:]
	}
	return c
}

func editable(n uiNode) bool {
	return strings.HasSuffix(n.Class, "EditText") || strings.HasSuffix(n.Class, "AutoCompleteTextView")
}

// Outline turns a uiautomator dump into an indented outline for an agent:
// a line per node that says something (text, description, id) or can be
// acted on, with [ref=mN] on what can be tapped, typed into or scrolled.
// refs maps each ref to its node's bounds.
func Outline(dump []byte) (outline string, refs map[string]Rect, err error) {
	start := strings.Index(string(dump), "<hierarchy")
	if start < 0 {
		return "", nil, errors.New("no UI hierarchy in the dump")
	}
	var h uiHierarchy
	if err := xml.Unmarshal(dump[start:], &h); err != nil {
		return "", nil, fmt.Errorf("parse UI dump: %w", err)
	}
	refs = map[string]Rect{}
	var b strings.Builder
	var walk func(n uiNode, depth int)
	walk = func(n uiNode, depth int) {
		r, ok := parseBounds(n.Bounds)
		act := n.Enabled && (n.Clickable || n.LongClick || n.Scrollable || editable(n) || n.Checkable)
		id := n.ResourceID
		if i := strings.LastIndex(id, ":id/"); i >= 0 {
			id = id[i+4:]
		}
		says := n.Text != "" || n.Desc != ""
		shown := ok && !r.Empty() && (says || act)
		if shown {
			b.WriteString(strings.Repeat("  ", depth))
			b.WriteString("- " + shortClass(n.Class))
			if n.Text != "" && !n.Password {
				fmt.Fprintf(&b, " %q", clip(n.Text, 200))
			}
			if n.Desc != "" && n.Desc != n.Text {
				fmt.Fprintf(&b, " (%s)", clip(n.Desc, 200))
			}
			if id != "" {
				b.WriteString(" #" + id)
			}
			if act {
				ref := "m" + strconv.Itoa(len(refs)+1)
				refs[ref] = r
				b.WriteString(" [ref=" + ref + "]")
			}
			for _, f := range []struct {
				on   bool
				word string
			}{{n.Checkable && n.Checked, "checked"}, {n.Selected, "selected"}, {n.Focused, "focused"},
				{n.Scrollable, "scrollable"}, {editable(n), "editable"}, {n.Password, "password"}} {
				if f.on {
					b.WriteString(" " + f.word)
				}
			}
			b.WriteByte('\n')
			depth++
		}
		for _, c := range n.Nodes {
			walk(c, depth)
		}
	}
	for _, n := range h.Nodes {
		walk(n, 0)
	}
	return b.String(), refs, nil
}

func clip(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
