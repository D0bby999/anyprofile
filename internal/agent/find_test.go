// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"testing"

	"github.com/D0bby999/anyprofile/internal/automation"
)

const findFixture = `Tab t1: Shop
URL: https://example.test

- button "Lưu" [ref=e41]
- button "Lưu nháp" [ref=e42] [disabled]
- link "Trang chủ" [ref=e5]
- textbox "Email" [ref=e7]
`

func TestFindMatches(t *testing.T) {
	ms := findMatches(findFixture, automation.Target{Text: "Lưu"}, 10)
	if len(ms) != 2 || ms[0].Ref != "e41" || ms[1].Ref != "e42" {
		t.Fatalf("text Lưu = %+v", ms)
	}
	if ms[0].Enabled != true || ms[1].Enabled != false {
		t.Fatalf("enabled flags = %+v", ms)
	}
	one := findMatches(findFixture, automation.Target{Text: "Lưu"}, 1)
	if len(one) != 1 || one[0].Ref != "e41" {
		t.Fatalf("limit 1 = %+v", one)
	}
}
