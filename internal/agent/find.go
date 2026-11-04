// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/automation"
)

// findLine matches a snapshot line that carries a ref:
//
//   - button "Lưu" [ref=e41] [disabled]
var findLine = regexp.MustCompile(`^ *- ([A-Za-z0-9_-]+)(?: "((?:\\.|[^"\\])*)")?.*\[ref=([^\]]+)\]`)

// Find returns elements matching t, best first, at most limit. It reads the
// same accessibility tree as Snapshot, so a ref it returns works with click.
func (b *Browser) Find(ctx context.Context, id string, t automation.Target, limit int) ([]automation.Match, error) {
	if t.Ref != "" {
		return []automation.Match{{Ref: t.Ref, Enabled: true}}, nil
	}
	if t.Point != nil {
		return []automation.Match{{Ref: "", X1: t.Point.X, Y1: t.Point.Y, X2: t.Point.X, Y2: t.Point.Y, Enabled: true}}, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	if t.TestID != "" {
		return b.findTestID(ctx, id, t.TestID, limit)
	}
	out, err := b.Snapshot(ctx, id, "")
	if err != nil {
		return nil, err
	}
	return findMatches(out, t, limit), nil
}

// findTestID resolves [data-testid="…"] in the page's top frame. Iframes are
// not searched; use text or role for those.
func (b *Browser) findTestID(ctx context.Context, id, testID string, limit int) ([]automation.Match, error) {
	t, err := b.tab(ctx, id)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Root struct {
			NodeID int `json:"nodeId"`
		} `json:"root"`
	}
	if err := b.call(ctx, t.session, "DOM.getDocument", map[string]any{"depth": 0}, &doc); err != nil {
		return nil, err
	}
	sel := `[data-testid="` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(testID) + `"]`
	var found struct {
		NodeIDs []int `json:"nodeIds"`
	}
	if err := b.call(ctx, t.session, "DOM.querySelectorAll", map[string]any{"nodeId": doc.Root.NodeID, "selector": sel}, &found); err != nil {
		return nil, err
	}
	var out []automation.Match
	for _, nodeID := range found.NodeIDs {
		var desc struct {
			Node struct {
				BackendNodeID int64 `json:"backendNodeId"`
			} `json:"node"`
		}
		if err := b.call(ctx, t.session, "DOM.describeNode", map[string]any{"nodeId": nodeID, "depth": 0}, &desc); err != nil {
			continue
		}
		out = append(out, automation.Match{Ref: "e" + strconv.FormatInt(desc.Node.BackendNodeID, 10), TestID: testID, Enabled: true})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// findMatches is the pure part of Find: it reads snapshot lines, so a test
// can feed a fixture instead of a browser.
func findMatches(outline string, t automation.Target, limit int) []automation.Match {
	type scored struct {
		m automation.Match
		s int
	}
	var found []scored
	for _, line := range strings.Split(outline, "\n") {
		m := findLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		role := m[1]
		name := unquote(m[2])
		score := t.Match(role, name, name, "", "")
		if score == 0 {
			continue
		}
		found = append(found, scored{
			m: automation.Match{Ref: m[3], Role: role, Name: name, Enabled: !strings.Contains(line, "[disabled]")},
			s: score,
		})
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].s != found[j].s {
			return found[i].s > found[j].s
		}
		return found[i].m.Ref < found[j].m.Ref
	})
	if len(found) > limit {
		found = found[:limit]
	}
	res := make([]automation.Match, 0, len(found))
	for _, f := range found {
		res = append(res, f.m)
	}
	return res
}

// FindOne returns the single match for t within five seconds, or an error.
// The wait is what lets a caller click a button that appears after the page
// has rendered, without a sleep.
func (b *Browser) FindOne(ctx context.Context, id string, t automation.Target) (automation.Match, error) {
	return b.FindOneWait(ctx, id, t, 5*time.Second)
}

// FindOneWait is FindOne with an explicit budget.
func (b *Browser) FindOneWait(ctx context.Context, id string, t automation.Target, wait time.Duration) (automation.Match, error) {
	deadline := time.Now().Add(wait)
	prev := ""
	var disabled *automation.Match
	for {
		ms, err := b.Find(ctx, id, t, 10)
		if err != nil {
			return automation.Match{}, err
		}
		switch len(ms) {
		case 1:
			if !ms[0].Enabled {
				disabled = &ms[0]
				prev = ""
				if time.Now().After(deadline) {
					return automation.Match{}, apperr.Newf("target_disabled", "%s is disabled (ref %s)", t, ms[0].Ref).Never()
				}
				break
			}
			// Two finds with the same ref mean the element stopped moving:
			// an opening menu or a carousel is not clicked mid-animation.
			if prev == ms[0].Ref {
				return ms[0], nil
			}
			prev = ms[0].Ref
			if time.Now().Add(150 * time.Millisecond).After(deadline) {
				return ms[0], nil
			}
		case 0:
			prev = ""
			if time.Now().After(deadline) {
				if disabled != nil {
					return automation.Match{}, apperr.Newf("target_disabled", "%s is disabled (ref %s)", t, disabled.Ref).Never()
				}
				return automation.Match{}, apperr.Newf("target_not_found", "no element matches %s after %s", t, wait).Safe()
			}
		default:
			var refs []string
			for _, m := range ms {
				refs = append(refs, m.Ref+" "+strconv.Quote(m.Name))
			}
			return automation.Match{}, apperr.Newf("target_ambiguous", "%s matches %d elements: %s", t, len(ms), strings.Join(refs, ", ")).Never()
		}
		select {
		case <-ctx.Done():
			return automation.Match{}, ctx.Err()
		case <-time.After(400 * time.Millisecond):
		}
	}
}

// unquote turns the captured name back into a string; a malformed escape is
// returned as it was written rather than failing the whole find.
func unquote(raw string) string {
	if raw == "" {
		return ""
	}
	if s, err := strconv.Unquote(`"` + raw + `"`); err == nil {
		return s
	}
	return raw
}
