// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package vm

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"golang.org/x/sys/unix"

	"github.com/D0bby999/anyprofile/internal/mobile/adb"
	"github.com/D0bby999/anyprofile/internal/mobile/vm/vnet"
	"github.com/D0bby999/anyprofile/internal/netproxy"
)

const (
	mindTheGappsName = "MindTheGapps-16.0.0-arm64-20261023_034858.zip"
	mindTheGappsURL  = "https://github.com/MindTheGapps/16.0.0-arm64/releases/download/MindTheGapps-16.0.0-arm64-20261023_034858/" + mindTheGappsName
	mindTheGappsSHA  = "6929593937cd85541228e4e5db9cbb5455338bf33c75eabf610de632ac155810"
)

// Shared helpers used by the macOS Metal mobile driver (avo) for window
// layout, network dialing, APFS cloning, GMS caching, and Android settings.
type WindowState = windowState
type WindowRect = windowRect

// MACFor gives each device a stable locally-administered network address.
func MACFor(id string) string {
	h := sha256.Sum256([]byte("anyprofile-mobile-mac:" + id))
	return fmt.Sprintf("52:54:00:%02x:%02x:%02x", h[0], h[1], h[2])
}

func directDial(ctx context.Context, addr string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "tcp", addr)
}

func offline(context.Context, string) (net.Conn, error) {
	return nil, errors.New("no network while the device is set up")
}

func DialerFor(up *netproxy.Upstream) vnet.Dialer {
	if up == nil {
		return directDial
	}
	u := *up
	return func(ctx context.Context, addr string) (net.Conn, error) { return u.DialTunnel(ctx, addr) }
}

func OfflineDialer() vnet.Dialer { return offline }

// SetAwake switches between windowed (stay on) and headless (screen off after 15 s).
func SetAwake(ctx context.Context, a adb.Client, on bool) {
	c, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if on {
		_, _ = a.Shell(c, "svc power stayon true")
		_, _ = a.Shell(c, "settings put system screen_off_timeout 2147483647")
		_, _ = a.Shell(c, "input keyevent KEYCODE_WAKEUP")
		return
	}
	_, _ = a.Shell(c, "svc power stayon false")
	_, _ = a.Shell(c, "settings put system screen_off_timeout 15000")
}

func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// CloneOrCopy clones src to dst on APFS, falling back to a byte copy on other volumes.
func CloneOrCopy(src, dst string) error {
	if err := unix.Clonefile(src, dst, unix.CLONE_NOFOLLOW); err == nil {
		return nil
	}
	return CopyFile(src, dst)
}

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
	timeout, stayon, anim := "2147483647", "true", "0.5"
	if headless {
		timeout, stayon, anim = "15000", "false", "0"
	} else if preset == "automation" {
		anim = "0"
	}
	script := fmt.Sprintf(
		"settings put global window_animation_scale %s\nsettings put global transition_animation_scale %s\nsettings put global animator_duration_scale %s\nsettings put system screen_off_timeout %s\nsvc power stayon %s",
		anim, anim, anim, timeout, stayon,
	)
	_, err := a.Shell(ctx, script)
	return err
}

func ValidPropValue(v string) bool { return validPropValue(v) }

func FetchMindTheGapps(ctx context.Context, cacheDir string) (string, error) {
	return adb.FetchAPK(ctx, cacheDir, mindTheGappsName, mindTheGappsURL, mindTheGappsSHA)
}
