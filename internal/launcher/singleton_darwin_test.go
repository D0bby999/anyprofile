// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"os"
	"path/filepath"
	"testing"
)

// Chromium on macOS names its owner in the SingletonLock symlink.
func TestSingletonPIDFromLock(t *testing.T) {
	udd := t.TempDir()
	os.Symlink("my-mac.local-4242", filepath.Join(udd, "SingletonLock"))
	if got := singletonPID(udd); got != 4242 {
		t.Fatalf("pid = %d", got)
	}
}
