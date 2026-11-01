// SPDX-License-Identifier: AGPL-3.0-or-later

package automation

import "testing"

func TestTargetMatch(t *testing.T) {
	cases := []struct {
		name string
		t    Target
		role string
		text string
		desc string
		id   string
		want int
	}{
		{"text exact", Target{Text: "Lưu"}, "button", "Lưu", "", "", 85},
		{"text contains", Target{Text: "Lưu"}, "button", "Lưu nháp", "", "", 60},
		{"role and name", Target{Role: "button", Name: "Lưu"}, "Button", "Lưu", "", "", 90},
		{"role mismatch", Target{Role: "link", Name: "Lưu"}, "button", "Lưu", "", "", 0},
		{"test id", Target{TestID: "save"}, "Button", "Lưu", "", "save", 100},
		{"test id mismatch", Target{TestID: "save"}, "Button", "Lưu", "", "cancel", 0},
		{"label", Target{Label: "Email"}, "textbox", "Email", "", "", 95},
		{"desc", Target{Desc: "Giỏ hàng"}, "Image", "", "Giỏ hàng", "", 95},
	}
	for _, c := range cases {
		if got := c.t.Match(c.role, c.text, c.text, c.desc, c.id); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

func TestTargetEmptyAndString(t *testing.T) {
	if !(Target{}).Empty() {
		t.Fatal("zero target should be empty")
	}
	if (Target{Text: "Lưu"}).Empty() {
		t.Fatal("text target should not be empty")
	}
	if got := (Target{Text: "Lưu"}).String(); got != `text "Lưu"` {
		t.Fatalf("String = %q", got)
	}
}
