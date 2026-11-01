// SPDX-License-Identifier: AGPL-3.0-or-later

package control

import (
	"context"
	"strings"
	"testing"
)

func TestGrantPermissionRejectsUnknownInput(t *testing.T) {
	d := &Device{}
	if err := d.GrantPermission(context.Background(), "com.test", "root"); err == nil || !strings.Contains(err.Error(), "mobile_permission") {
		t.Fatalf("bad permission err = %v", err)
	}
	if err := d.GrantPermission(context.Background(), "not a package", "camera"); err == nil || !strings.Contains(err.Error(), "mobile_package") {
		t.Fatalf("bad package err = %v", err)
	}
}
