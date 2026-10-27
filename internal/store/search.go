// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"database/sql/driver"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
	"modernc.org/sqlite"
)

// foldSearch mirrors frontend/lib/format.fold: NFD, remove combining marks,
// map Vietnamese đ to d, then lowercase. The deterministic SQL function is
// registered for every connection, including databases restored from backup.
func foldSearch(s string) string {
	return strings.ToLower(strings.Map(func(r rune) rune {
		if unicode.Is(unicode.M, r) {
			return -1
		}
		if r == 'đ' || r == 'Đ' {
			return 'd'
		}
		return r
	}, norm.NFD.String(s)))
}

func init() {
	sqlite.MustRegisterDeterministicScalarFunction("anyprofile_fold", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		if args[0] == nil {
			return "", nil
		}
		s, _ := args[0].(string)
		return foldSearch(s), nil
	})
}
