// SPDX-License-Identifier: AGPL-3.0-or-later

// Package apperr gives errors the UI translates a stable code. Error()
// renders as "anyprofile:<code>: <English message>".
package apperr

import "fmt"

type Error struct {
	Code string
	Msg  string
}

func (e *Error) Error() string { return "anyprofile:" + e.Code + ": " + e.Msg }

func New(code, msg string) *Error { return &Error{Code: code, Msg: msg} }

func Newf(code, format string, a ...any) *Error {
	return &Error{Code: code, Msg: fmt.Sprintf(format, a...)}
}
