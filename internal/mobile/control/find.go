// SPDX-License-Identifier: AGPL-3.0-or-later

package control

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

// findLine matches an outline line with a ref:
//
//   - Button "Lưu" #save [ref=m1] [checked]
var findLine = regexp.MustCompile(`^ *- ([A-Za-z0-9_]+)(?: "((?:\\.|[^"\\])*)")?(?: \(([^)]*)\))?(?: #(\S+))?.*\[ref=(m\d+)\]`)

// Find returns elements matching t, best first. It reads the same uiautomator
// tree as Snapshot and registers the refs, so a returned ref works with Tap.
func (d *Device) Find(ctx context.Context, t automation.Target, limit int) ([]automation.Match, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	if t.Ref != "" {
		d.mu.Lock()
		r, ok := d.refs[t.Ref]
		d.mu.Unlock()
		if !ok {
			return nil, apperr.Newf("mobile_ref", "%q is not on the screen any more; take a new find or snapshot", t.Ref)
		}
		return []automation.Match{{Ref: t.Ref, Enabled: true, X1: r.X1, Y1: r.Y1, X2: r.X2, Y2: r.Y2}}, nil
	}
	if t.Point != nil {
		return []automation.Match{{X1: t.Point.X, Y1: t.Point.Y, X2: t.Point.X, Y2: t.Point.Y, Enabled: true}}, nil
	}

	d.wake(ctx)
	out, err := d.shell(ctx, "uiautomator dump "+dumpPath+" >/dev/null 2>&1 && cat "+dumpPath+"; rm -f "+dumpPath)
	if err != nil || !strings.Contains(out, "<hierarchy") {
		return nil, apperr.New("mobile_find", "the screen keeps moving (like a video feed) and cannot be read as text; use device_screenshot and tap by x and y")
	}
	outline, refs, err := Outline([]byte(out))
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	d.refs = refs
	d.mu.Unlock()
	return findMatches(outline, refs, t, limit), nil
}

// findMatches is the pure part of Find: it reads outline lines and the refs
// map, so a test can feed a fixture instead of a device.
func findMatches(outline string, refs map[string]Rect, t automation.Target, limit int) []automation.Match {
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
		name := unquoteFind(m[2])
		desc := m[3]
		testID := m[4]
		score := t.Match(role, name, name, desc, testID)
		if score == 0 {
			continue
		}
		r := refs[m[5]]
		found = append(found, scored{
			m: automation.Match{Ref: m[5], Role: role, Name: name, TestID: testID, Enabled: true,
				X1: r.X1, Y1: r.Y1, X2: r.X2, Y2: r.Y2},
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

// FindOne returns the single match within five seconds; an element that
// appears after a transition is found without a sleep.
func (d *Device) FindOne(ctx context.Context, t automation.Target) (automation.Match, error) {
	return d.FindOneWait(ctx, t, 5*time.Second)
}

// FindOneWait is FindOne with an explicit budget.
func (d *Device) FindOneWait(ctx context.Context, t automation.Target, wait time.Duration) (automation.Match, error) {
	deadline := time.Now().Add(wait)
	prev := ""
	for {
		ms, err := d.Find(ctx, t, 10)
		if err != nil {
			return automation.Match{}, err
		}
		switch len(ms) {
		case 1:
			// Two finds with the same ref mean the element stopped moving
			// (a transition, a keyboard opening, a list settling).
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
				return automation.Match{}, apperr.Newf("target_not_found", "no element matches %s after %s", t, wait)
			}
		default:
			var refs []string
			for _, m := range ms {
				refs = append(refs, m.Ref+" "+strconv.Quote(m.Name))
			}
			return automation.Match{}, apperr.Newf("target_ambiguous", "%s matches %d elements: %s", t, len(ms), strings.Join(refs, ", "))
		}
		select {
		case <-ctx.Done():
			return automation.Match{}, ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
}

func unquoteFind(raw string) string {
	if raw == "" {
		return ""
	}
	if s, err := strconv.Unquote(`"` + raw + `"`); err == nil {
		return s
	}
	return raw
}
