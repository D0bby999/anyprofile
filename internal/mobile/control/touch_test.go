// SPDX-License-Identifier: AGPL-3.0-or-later

package control

import (
	"context"
	"strings"
	"testing"
)

func TestPullRejectsPathsOutsideTheAllowlist(t *testing.T) {
	d := &Device{}
	for _, p := range []string{"/etc/passwd", "/sdcard/DCIM/AnyProfile/../../etc/passwd", "relative/file"} {
		if _, err := d.Pull(context.Background(), p); err == nil ||
			!strings.Contains(err.Error(), "mobile_pull_path") {
			t.Errorf("Pull(%q) err = %v, want mobile_pull_path", p, err)
		}
	}
}
