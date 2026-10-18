// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

// A ref names an element by its backend DOM node id: "e123" in the tab's
// page, "e123f2" in the second iframe a snapshot reached. Sign-in boxes and
// captchas live in iframes, often of another site, which Chromium runs in a
// process of their own (an OOPIF) with its own DevTools session.

// frameSess is an iframe of a tab whose contents a snapshot reached.
type frameSess struct {
	id      string     // frame id; for an OOPIF also its target id
	session string     // the tab's own session for a frame in the page's process
	owner   int64      // the <iframe> element, in the parent's session
	parent  *frameSess // nil: the page
	oopif   bool       // own process: own session, coordinates from its own corner
	worldID int        // isolated world of the agent in it, 0 when none yet
}

func (f *frameSess) parentSession(t *tab) string {
	if f.parent == nil {
		return t.session
	}
	return f.parent.session
}

// resolve parses ref into the node and the frame it is in (nil: the page).
func (b *Browser) resolve(t *tab, ref string) (int64, *frameSess, error) {
	bad := apperr.Newf("agent_bad_ref", "%q is not a ref from snapshot (like e123)", ref)
	if !strings.HasPrefix(ref, "e") {
		return 0, nil, bad
	}
	nodePart, framePart, inFrame := strings.Cut(ref[1:], "f")
	node, err := strconv.ParseInt(nodePart, 10, 64)
	if err != nil || node <= 0 {
		return 0, nil, bad
	}
	if !inFrame {
		return node, nil, nil
	}
	k, err := strconv.Atoi(framePart)
	if err != nil || k <= 0 {
		return 0, nil, bad
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if k > len(t.frames) {
		return 0, nil, errGone(ref)
	}
	return node, t.frames[k-1], nil
}

// sessionOf is the DevTools session that holds f's nodes.
func sessionOf(t *tab, f *frameSess) string {
	if f == nil {
		return t.session
	}
	return f.session
}

// refFor writes the ref of node in f.
func (t *tab) refFor(node int64, f *frameSess) string {
	if f == nil {
		return "e" + strconv.FormatInt(node, 10)
	}
	for i, g := range t.frames {
		if g == f {
			return fmt.Sprintf("e%df%d", node, i+1)
		}
	}
	return "e" + strconv.FormatInt(node, 10)
}

// frameOf returns the frame an <iframe> element (owner, in session of
// parent) shows, reaching into it: in the same process its tree is asked of
// the parent's session; in another, the agent attaches to it.
func (b *Browser) frameOf(ctx context.Context, t *tab, parent *frameSess, owner int64) (*frameSess, []axNode, error) {
	session := sessionOf(t, parent)
	var d struct {
		Node struct {
			FrameID string `json:"frameId"`
		} `json:"node"`
	}
	if err := b.call(ctx, session, "DOM.describeNode", map[string]any{"backendNodeId": owner}, &d); err != nil {
		return nil, nil, err
	}
	if d.Node.FrameID == "" {
		return nil, nil, fmt.Errorf("the iframe has no document yet")
	}
	b.mu.Lock()
	var f *frameSess
	for _, g := range t.frames {
		if g.id == d.Node.FrameID {
			f = g
		}
	}
	b.mu.Unlock()
	var r struct {
		Nodes []axNode `json:"nodes"`
	}
	if f == nil || !f.oopif {
		if err := b.call(ctx, session, "Accessibility.getFullAXTree", map[string]any{"frameId": d.Node.FrameID}, &r); err == nil {
			f = b.keepFrame(t, &frameSess{id: d.Node.FrameID, session: session, owner: owner, parent: parent})
			return f, r.Nodes, nil
		}
	}
	if f == nil {
		var a struct {
			SessionID string `json:"sessionId"`
		}
		if err := b.call(ctx, "", "Target.attachToTarget", map[string]any{"targetId": d.Node.FrameID, "flatten": true}, &a); err != nil {
			return nil, nil, err
		}
		b.call(ctx, a.SessionID, "Page.enable", map[string]any{}, nil) // for dialogs in it
		f = b.keepFrame(t, &frameSess{id: d.Node.FrameID, session: a.SessionID, owner: owner, parent: parent, oopif: true})
	}
	if err := b.call(ctx, f.session, "Accessibility.getFullAXTree", map[string]any{}, &r); err != nil {
		return nil, nil, err
	}
	return f, r.Nodes, nil
}

// keepFrame adds f to the tab's frames, or updates the one with its id, so
// refs from an earlier snapshot keep their number.
func (b *Browser) keepFrame(t *tab, f *frameSess) *frameSess {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, g := range t.frames {
		if g.id == f.id {
			if g.session != f.session {
				g.worldID = 0
			}
			g.session, g.owner, g.parent, g.oopif = f.session, f.owner, f.parent, f.oopif
			return g
		}
	}
	t.frames = append(t.frames, f)
	return f
}

// frameOffset is where f's viewport starts in the page's viewport. Content
// of a frame in the page's process is placed in page coordinates already;
// an OOPIF's starts at its own corner, which is its <iframe>'s content box.
func (b *Browser) frameOffset(ctx context.Context, t *tab, f *frameSess) (float64, float64, error) {
	if f == nil {
		return 0, 0, nil
	}
	px, py, err := b.frameOffset(ctx, t, f.parent)
	if err != nil || !f.oopif {
		return px, py, err
	}
	var m struct {
		Model struct {
			Content []float64 `json:"content"`
		} `json:"model"`
	}
	if err := b.call(ctx, f.parentSession(t), "DOM.getBoxModel", map[string]any{"backendNodeId": f.owner}, &m); err != nil {
		return 0, 0, err
	}
	if len(m.Model.Content) < 2 {
		return 0, 0, fmt.Errorf("the iframe has no box")
	}
	return px + m.Model.Content[0], py + m.Model.Content[1], nil
}

// frameWorld is the agent's isolated world in f (nil: the page).
func (b *Browser) frameWorld(ctx context.Context, t *tab, f *frameSess) (int, error) {
	if f == nil {
		return b.world(ctx, t)
	}
	b.mu.Lock()
	id := f.worldID
	b.mu.Unlock()
	if id != 0 {
		return id, nil
	}
	var r struct {
		ExecutionContextID int `json:"executionContextId"`
	}
	if err := b.call(ctx, f.session, "Page.createIsolatedWorld", map[string]any{"frameId": f.id, "worldName": "anyprofile"}, &r); err != nil {
		return 0, err
	}
	b.mu.Lock()
	f.worldID = r.ExecutionContextID
	b.mu.Unlock()
	return r.ExecutionContextID, nil
}

// subtree is the snapshot outline of a frame, its own iframes included.
func (b *Browser) subtree(ctx context.Context, t *tab, f *frameSess, nodes []axNode, root string, limit int) string {
	if root == "" && len(nodes) > 0 {
		root = nodes[0].NodeID
	}
	return outline(nodes, root, limit, func(n int64) string { return t.refFor(n, f) },
		func(owner int64, depth int) string {
			g, inner, err := b.frameOf(ctx, t, f, owner)
			if err != nil || len(inner) == 0 {
				return ""
			}
			return indent(b.subtree(ctx, t, g, inner, "", limit), depth)
		})
}

func indent(s string, depth int) string {
	if s == "" || depth == 0 {
		return s
	}
	pad := strings.Repeat("  ", depth)
	lines := strings.SplitAfter(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = pad + l
		}
	}
	return strings.Join(lines, "")
}
