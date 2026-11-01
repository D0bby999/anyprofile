// SPDX-License-Identifier: AGPL-3.0-or-later

// Package automation holds the target model shared by the browser and mobile
// surfaces. A target names an element the way a person would (text, role and
// name, label, test id) so a flow survives a re-render; a ref stays the fast
// path when one is already known.
package automation

import (
	"strconv"
	"strings"
)

// Target is what to act on. All fields are optional, but at least one must be
// set. Resolution order: Ref, TestID, Label/Desc, Role+Name, Text, Point.
type Target struct {
	Ref    string `json:"ref,omitempty" jsonschema:"element ref from snapshot/find, like e123 or m12"`
	Text   string `json:"text,omitempty" jsonschema:"visible text, exact first then contains"`
	Role   string `json:"role,omitempty" jsonschema:"role or class, like button or Button"`
	Name   string `json:"name,omitempty" jsonschema:"accessible name used with role"`
	TestID string `json:"testId,omitempty" jsonschema:"data-testid (web) or resource-id (mobile)"`
	Label  string `json:"label,omitempty" jsonschema:"form label or aria-label"`
	Desc   string `json:"desc,omitempty" jsonschema:"content description (mobile)"`
	Point  *Point `json:"point,omitempty" jsonschema:"last resort: absolute screen point"`
}

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Match is one candidate a surface found.
type Match struct {
	Ref     string `json:"ref"`
	Role    string `json:"role"`
	Name    string `json:"name"`
	TestID  string `json:"testId,omitempty"`
	Enabled bool   `json:"enabled"`
	X1      int    `json:"x1,omitempty"`
	Y1      int    `json:"y1,omitempty"`
	X2      int    `json:"x2,omitempty"`
	Y2      int    `json:"y2,omitempty"`
}

// Empty reports a target that asks for nothing.
func (t Target) Empty() bool {
	return t.Ref == "" && t.Text == "" && t.Role == "" && t.Name == "" &&
		t.TestID == "" && t.Label == "" && t.Desc == "" && t.Point == nil
}

// Match scores an element against the target. A higher score is a better
// match; 0 means no match. The shared scoring keeps web and mobile consistent.
func (t Target) Match(role, name, text, desc, testID string) int {
	if t.Ref != "" {
		return 0 // refs are resolved by the surface, not by matching
	}
	if t.TestID != "" {
		if testID == "" || !strings.EqualFold(testID, t.TestID) {
			return 0
		}
		return 100
	}
	if t.Point != nil {
		return 50 // resolved by the surface
	}
	role = strings.ToLower(role)
	name = strings.TrimSpace(name)
	text = strings.TrimSpace(text)
	desc = strings.TrimSpace(desc)
	wantRole := strings.ToLower(t.Role)

	if t.Label != "" {
		if eqFold(name, t.Label) {
			return 95
		}
		if containsFold(name, t.Label) {
			return 70
		}
	}
	if t.Desc != "" {
		if eqFold(desc, t.Desc) {
			return 95
		}
		if containsFold(desc, t.Desc) {
			return 70
		}
	}
	if t.Role != "" || t.Name != "" {
		if wantRole != "" && role != wantRole {
			return 0
		}
		if t.Name != "" {
			switch {
			case eqFold(name, t.Name):
				return 90
			case containsFold(name, t.Name):
				return 65
			default:
				return 0
			}
		}
		return 55
	}
	if t.Text != "" {
		switch {
		case eqFold(text, t.Text), eqFold(name, t.Text):
			return 85
		case containsFold(text, t.Text), containsFold(name, t.Text):
			return 60
		default:
			return 0
		}
	}
	return 0
}

// String makes errors read like "text \"Lưu\"".
func (t Target) String() string {
	switch {
	case t.Text != "":
		return "text " + strconv.Quote(t.Text)
	case t.Role != "" && t.Name != "":
		return t.Role + " " + strconv.Quote(t.Name)
	case t.Name != "":
		return "name " + strconv.Quote(t.Name)
	case t.Label != "":
		return "label " + strconv.Quote(t.Label)
	case t.TestID != "":
		return "testId " + strconv.Quote(t.TestID)
	case t.Ref != "":
		return t.Ref
	default:
		return "target"
	}
}

func eqFold(a, b string) bool { return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)) }

func containsFold(a, b string) bool {
	return strings.Contains(strings.ToLower(a), strings.ToLower(strings.TrimSpace(b)))
}
