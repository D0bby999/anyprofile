// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"strings"
	"testing"
)

func TestInteractiveKeepsRefsAndHead(t *testing.T) {
	in := `Tab t1: Shop
URL: https://shop.example/

- button "Lưu" [ref=e41]
  - StaticText "mô tả dài"
- link "Trang chủ" [ref=e5]
`
	out := Interactive(in)
	if !strings.Contains(out, "Tab t1") || !strings.Contains(out, "ref=e41") || !strings.Contains(out, "ref=e5") {
		t.Fatalf("interactive = %q", out)
	}
	if strings.Contains(out, "mô tả dài") {
		t.Fatalf("static text kept: %q", out)
	}
}
