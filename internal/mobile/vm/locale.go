// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package vm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/adb"
)

// Measured on LineageOS 23.2 (Android 16) without root:
//   - the timezone: turn off automatic detection, then `cmd alarm set-timezone`;
//   - the language: the system language needs root (persist.sys.locale), so
//     each app gets it as its own language (`cmd locale set-app-locales`,
//     Android 13+): every installed app and the built-in browser. An app
//     installed while the device runs gets it at the next start.
const browserPkg = "org.lineageos.jelly"

// localeScript is the shell line that sets tz and locale on pkgs. Every value
// was checked against a strict pattern, so none can reach the shell as code.
func localeScript(tz, locale string, pkgs []string) (string, error) {
	var cmds []string
	if tz != "" {
		if !mobile.ValidTimezone(tz) {
			return "", fmt.Errorf("bad timezone %q", tz)
		}
		cmds = append(cmds, "cmd time_zone_detector set_auto_detection_enabled false", "cmd alarm set-timezone "+tz)
	}
	if locale != "" {
		if !mobile.ValidLocale(locale) {
			return "", fmt.Errorf("bad locale %q", locale)
		}
		for _, p := range pkgs {
			if mobile.ValidPackage(p) {
				cmds = append(cmds, "cmd locale set-app-locales "+p+" --locales "+locale)
			}
		}
	}
	return strings.Join(cmds, "; "), nil
}

// thirdPartyPackages lists the apps the user installed.
func thirdPartyPackages(ctx context.Context, a adb.Client) ([]string, error) {
	out, err := a.Shell(ctx, "pm list packages -3")
	if err != nil {
		return nil, err
	}
	var pkgs []string
	for _, l := range strings.Split(out, "\n") {
		if p, ok := strings.CutPrefix(strings.TrimSpace(l), "package:"); ok {
			pkgs = append(pkgs, p)
		}
	}
	return pkgs, nil
}

func applyLocale(ctx context.Context, a adb.Client, tz, locale string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	pkgs, err := thirdPartyPackages(ctx, a)
	if err != nil {
		return err
	}
	line, err := localeScript(tz, locale, append(pkgs, browserPkg))
	if err != nil || line == "" {
		return err
	}
	_, err = a.Shell(ctx, line)
	return err
}
