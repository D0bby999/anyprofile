// SPDX-License-Identifier: AGPL-3.0-or-later

package fury

import "errors"

// clone has no Windows equivalent here; the caller copies instead.
func clone(string, string) error { return errors.New("no file clone on Windows") }
