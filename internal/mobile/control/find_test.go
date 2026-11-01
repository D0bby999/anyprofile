// SPDX-License-Identifier: AGPL-3.0-or-later

package control

import (
	"testing"

	"github.com/D0bby999/anyprofile/internal/automation"
)

const findFixture = `- Button "Lưu" #save [ref=m1]
- Button "Lưu nháp" [ref=m2]
- EditText "Email" #email [ref=m3] editable
`

func TestFindMatches(t *testing.T) {
	refs := map[string]Rect{"m1": {10, 20, 110, 80}, "m2": {10, 90, 110, 150}, "m3": {10, 160, 110, 220}}
	ms := findMatches(findFixture, refs, automation.Target{Text: "Lưu"}, 10)
	if len(ms) != 2 || ms[0].Ref != "m1" || ms[1].Ref != "m2" {
		t.Fatalf("text Lưu = %+v", ms)
	}
	if ms[0].X1 != 10 || ms[0].Y2 != 80 {
		t.Fatalf("bounds = %+v", ms[0])
	}
	if one := findMatches(findFixture, refs, automation.Target{TestID: "save"}, 10); len(one) != 1 || one[0].Ref != "m1" {
		t.Fatalf("testId save = %+v", one)
	}
	if one := findMatches(findFixture, refs, automation.Target{Text: "Lưu"}, 1); len(one) != 1 {
		t.Fatalf("limit 1 = %+v", one)
	}
}
