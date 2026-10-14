// SPDX-License-Identifier: AGPL-3.0-or-later

package paths

import (
	"strings"
	"testing"
)

func TestDefaultHonoursAbsoluteOverride(t *testing.T) {
	t.Setenv(EnvDataDir, "/tmp/ap-test-root/")
	d, err := Default()
	if err != nil || d.Root != "/tmp/ap-test-root" {
		t.Fatalf("override = %q %v", d.Root, err)
	}
	t.Setenv(EnvDataDir, "relative/dir")
	d, err = Default()
	if err != nil || !strings.HasSuffix(d.Root, "/AnyProfile") {
		t.Fatalf("relative override must be ignored, got %q %v", d.Root, err)
	}
}
