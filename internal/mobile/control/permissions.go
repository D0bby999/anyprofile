// SPDX-License-Identifier: AGPL-3.0-or-later

package control

import (
	"context"
	"strings"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/mobile"
)

// Android permissions an agent may grant. A fixed allowlist keeps a typo or a
// prompt-injected package from granting something broad.
var grantable = map[string]string{
	"camera":        "android.permission.CAMERA",
	"microphone":    "android.permission.RECORD_AUDIO",
	"location":      "android.permission.ACCESS_FINE_LOCATION",
	"notifications": "android.permission.POST_NOTIFICATIONS",
	"photos":        "android.permission.READ_MEDIA_IMAGES",
}

// GrantPermission grants one allowlisted permission to an installed package.
func (d *Device) GrantPermission(ctx context.Context, pkg, permission string) error {
	perm, err := grantablePermission(permission)
	if err != nil {
		return err
	}
	if !mobile.ValidPackage(pkg) {
		return apperr.New("mobile_package", "that is not an app package name; see device_list_apps")
	}
	_, err = d.shell(ctx, "pm grant "+pkg+" "+perm)
	return err
}

// RevokePermission removes one allowlisted permission.
func (d *Device) RevokePermission(ctx context.Context, pkg, permission string) error {
	perm, err := grantablePermission(permission)
	if err != nil {
		return err
	}
	if !mobile.ValidPackage(pkg) {
		return apperr.New("mobile_package", "that is not an app package name; see device_list_apps")
	}
	_, err = d.shell(ctx, "pm revoke "+pkg+" "+perm)
	return err
}

func grantablePermission(name string) (string, error) {
	p, ok := grantable[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return "", apperr.New("mobile_permission", "permission is camera, microphone, location, notifications or photos")
	}
	return p, nil
}
