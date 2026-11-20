// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package vm

import (
	"context"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile/adb"
	"github.com/D0bby999/anyprofile/internal/mobile/vm/vnet"
	"github.com/D0bby999/anyprofile/internal/netproxy"
)

// Shared helpers exposed for sibling macOS mobile drivers (such as avo) so
// window layout, network dialing, cloning, GMS caching, and Android settings reuse one copy.
type WindowState = windowState
type WindowRect = windowRect

func MACFor(id string) string                     { return macFor(id) }
func DialerFor(up *netproxy.Upstream) vnet.Dialer { return dialerFor(up) }
func OfflineDialer() vnet.Dialer                  { return offline }
func SetAwake(ctx context.Context, a adb.Client, on bool) {
	setAwake(ctx, a, on)
}
func CloneOrCopy(src, dst string) error       { return cloneOrCopy(src, dst) }
func CopyFile(src, dst string) error          { return copyFile(src, dst) }
func ScreenWorkArea() (WindowRect, bool)      { return screenWorkArea() }
func DefaultWindow(n, sw, sh int) WindowState { return defaultWindow(n, sw, sh) }
func ArrangeLayout(a WindowRect, layout string, count, sw, sh int) []WindowState {
	return arrangeLayout(a, layout, count, sw, sh)
}
func SetClock(ctx context.Context, a adb.Client, now time.Time) error {
	return setClock(ctx, a, now)
}
func ApplyLocale(ctx context.Context, a adb.Client, tz, locale string) error {
	return applyLocale(ctx, a, tz, locale)
}
func ApplyAutomationSettings(ctx context.Context, a adb.Client, preset string, headless bool) error {
	return applyAutomationSettings(ctx, a, preset, headless)
}
func ValidPropValue(v string) bool { return validPropValue(v) }
func FetchMindTheGapps(ctx context.Context, cacheDir string) (string, error) {
	return adb.FetchAPK(ctx, cacheDir, mindTheGappsName, mindTheGappsURL, mindTheGappsSHA)
}
