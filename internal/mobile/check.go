// SPDX-License-Identifier: AGPL-3.0-or-later

package mobile

import "regexp"

// Values that drivers put on an adb shell command line are checked against
// these first, so none can reach the shell as code.
var (
	tzRe     = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_+\-]*(/[A-Za-z0-9_+\-]+)*$`)
	localeRe = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)
	pkgRe    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)+$`)
)

// ValidTimezone is an IANA zone name like "Asia/Ho_Chi_Minh".
func ValidTimezone(s string) bool { return tzRe.MatchString(s) }

// ValidLocale is a BCP 47 tag like "vi-VN".
func ValidLocale(s string) bool { return localeRe.MatchString(s) }

// ValidPackage is an Android package name like "com.zhiliaoapp.musically".
func ValidPackage(s string) bool { return pkgRe.MatchString(s) }
