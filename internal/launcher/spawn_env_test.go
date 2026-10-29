// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"slices"
	"testing"
)

func TestFilteredEnvRemovesInheritedKeys(t *testing.T) {
	parent := []string{"PATH=/bin", "GOOGLE_API_KEY=secret", "google_default_client_id=secret", "GOOGLE_DEFAULT_CLIENT_SECRET=secret", "GOOGLE_API_KEY_EXTRA=keep", "TZ=UTC"}
	unset := []string{"GOOGLE_API_KEY", "GOOGLE_DEFAULT_CLIENT_ID", "GOOGLE_DEFAULT_CLIENT_SECRET"}
	want := []string{"PATH=/bin", "GOOGLE_API_KEY_EXTRA=keep", "TZ=UTC"}
	if got := filteredEnv(parent, unset); !slices.Equal(got, want) {
		t.Fatalf("filtered env = %v, want %v", got, want)
	}
	if len(parent) != 6 {
		t.Fatal("parent environment changed")
	}
}
