// SPDX-License-Identifier: AGPL-3.0-or-later

package control

import (
	"context"
	"strings"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

// ClipboardGet returns the device clipboard. Android only exposes it to adb
// on some builds; elsewhere the caller gets a typed error, never an empty
// string that looks like success.
func (d *Device) ClipboardGet(ctx context.Context) (string, error) {
	d.wake(ctx)
	if out, err := d.shell(ctx, "cmd clipboard get 2>/dev/null"); err == nil && strings.TrimSpace(out) != "" && !strings.Contains(out, "Unknown command") {
		return out, nil
	}
	return "", apperr.New("clipboard_unsupported", "this Android build does not expose the clipboard to adb; copy on the device screen instead")
}

// ClipboardSet is not supported: Android 10+ gives adb no write path to the
// clipboard. Returning a typed error is better than a silent no-op.
func (d *Device) ClipboardSet(context.Context, string) error {
	return apperr.New("clipboard_unsupported", "this Android build does not let adb write the clipboard; type the text into the field instead")
}
