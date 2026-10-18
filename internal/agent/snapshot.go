// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// MaxText caps what one snapshot or read returns, in characters.
const MaxText = 40000

type axValue struct {
	Value json.RawMessage `json:"value"`
}

func (v *axValue) str() string {
	if v == nil || len(v.Value) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(v.Value, &s) == nil {
		return s
	}
	return strings.Trim(string(v.Value), `"`)
}

type axNode struct {
	NodeID     string   `json:"nodeId"`
	Ignored    bool     `json:"ignored"`
	Role       *axValue `json:"role"`
	Name       *axValue `json:"name"`
	Value      *axValue `json:"value"`
	Properties []struct {
		Name  string  `json:"name"`
		Value axValue `json:"value"`
	} `json:"properties"`
	ChildIDs         []string `json:"childIds"`
	BackendDOMNodeID int64    `json:"backendDOMNodeId"`
}

// Roles a person acts on; they get a ref.
var actionable = map[string]bool{
	"button": true, "link": true, "textbox": true, "searchbox": true, "combobox": true, "checkbox": true,
	"radio": true, "switch": true, "menuitem": true, "menuitemcheckbox": true, "menuitemradio": true,
	"tab": true, "option": true, "slider": true, "spinbutton": true, "listbox": true, "treeitem": true,
}

// Roles that only wrap others; their children are shown in their place.
var wrappers = map[string]bool{
	"generic": true, "none": true, "presentation": true, "group": true, "LayoutTable": true,
	"LayoutTableRow": true, "LayoutTableCell": true, "Section": true, "Iframe": true,
}

// Shown states, in this order.
var states = []string{"checked", "selected", "expanded", "pressed", "disabled", "required", "focused", "level"}

// Snapshot returns the page as an outline of roles and names. Elements a person
// acts on carry [ref=eN] for click and type. With ref, only that part.
func (b *Browser) Snapshot(ctx context.Context, id, ref string) (string, error) {
	t, err := b.tab(ctx, id)
	if err != nil {
		return "", err
	}
	var f *frameSess
	var want int64
	if ref != "" {
		if want, f, err = b.resolve(t, ref); err != nil {
			return "", err
		}
	}
	params := map[string]any{}
	if f != nil && !f.oopif {
		params["frameId"] = f.id
	}
	var r struct {
		Nodes []axNode `json:"nodes"`
	}
	if err := b.call(ctx, sessionOf(t, f), "Accessibility.getFullAXTree", params, &r); err != nil {
		if ref != "" {
			return "", errGone(ref)
		}
		return "", err
	}
	root := ""
	if ref != "" {
		for _, n := range r.Nodes {
			if n.BackendDOMNodeID == want {
				root = n.NodeID
				break
			}
		}
		if root == "" {
			return "", errGone(ref)
		}
	}
	body := b.subtree(ctx, t, f, r.Nodes, root, MaxText)
	page := b.describe(ctx, t)
	head := fmt.Sprintf("Tab %s: %s\nURL: %s\n", page.ID, page.Title, page.URL)
	if page.Dialog != "" {
		head += "Dialog: " + page.Dialog + "\n"
	}
	return head + "\n" + body, nil
}

// outline renders the tree under root as indented lines, at most limit
// characters.
// ref writes an element's ref; frame, when not nil, renders the frame an
// <iframe> element shows, at depth.
func outline(nodes []axNode, root string, limit int, ref func(int64) string, frame func(owner int64, depth int) string) string {
	if ref == nil {
		ref = func(n int64) string { return "e" + strconv.FormatInt(n, 10) }
	}
	byID := make(map[string]*axNode, len(nodes))
	for i := range nodes {
		byID[nodes[i].NodeID] = &nodes[i]
	}
	var sb strings.Builder
	left := 0
	var walk func(id, parentName string, depth int)
	walk = func(id, parentName string, depth int) {
		n := byID[id]
		if n == nil {
			return
		}
		if sb.Len() > limit {
			left++
			return
		}
		role, name := n.Role.str(), clean(n.Name.str())
		if role == "InlineTextBox" || role == "LineBreak" {
			return
		}
		focusable := false
		for _, p := range n.Properties {
			if p.Name == "focusable" && p.Value.str() == "true" {
				focusable = true
			}
		}
		hasRef := n.BackendDOMNodeID != 0 && !n.Ignored && (actionable[role] || (focusable && name != ""))
		skip := n.Ignored || role == "RootWebArea" || (wrappers[role] && !hasRef && name == "") ||
			(role == "StaticText" && (name == "" || name == parentName))
		next := depth
		if !skip {
			sb.WriteString(strings.Repeat("  ", depth))
			if role == "StaticText" {
				sb.WriteString("- text: " + strconv.Quote(name))
			} else {
				sb.WriteString("- " + role)
				if name != "" {
					sb.WriteString(" " + strconv.Quote(name))
				}
				if hasRef {
					sb.WriteString(" [ref=" + ref(n.BackendDOMNodeID) + "]")
				}
				for _, s := range states {
					for _, p := range n.Properties {
						if p.Name != s {
							continue
						}
						switch v := p.Value.str(); {
						case s == "level":
							sb.WriteString(" [level=" + v + "]")
						case v == "true" || v == "mixed":
							sb.WriteString(" [" + s + "]")
						}
					}
				}
				if v := clean(n.Value.str()); v != "" && v != name {
					sb.WriteString(" [value=" + strconv.Quote(v) + "]")
				}
			}
			sb.WriteByte('\n')
			next = depth + 1
			parentName = name
		}
		for _, c := range n.ChildIDs {
			walk(c, parentName, next)
		}
		if role == "Iframe" && frame != nil && n.BackendDOMNodeID != 0 && sb.Len() <= limit {
			sb.WriteString(frame(n.BackendDOMNodeID, next))
		}
	}
	walk(root, "", 0)
	out := sb.String()
	if left > 0 {
		out += fmt.Sprintf("… cut at %d characters; %d more elements. Snapshot one part with ref, or use read_page.\n", limit, left)
	}
	return out
}

// clean folds whitespace and caps a name at 200 characters.
func clean(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 200 {
		s = string(r[:200]) + "…"
	}
	return s
}
