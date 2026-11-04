// SPDX-License-Identifier: AGPL-3.0-or-later

// Package apperr gives errors the UI translates a stable code. Error()
// renders as "anyprofile:<code>: <English message>".
package apperr

import "fmt"

// What a caller may do next. An empty Retry says nothing: the caller treats
// the error as it always has, and an agent does not repeat the request.
const (
	// RetrySafe means the same request can succeed later, often after the
	// step the message names (a new snapshot, opening the profile).
	RetrySafe = "safe"
	// RetryNever means repeating it as written cannot succeed.
	RetryNever = "never"
)

type Error struct {
	Code string
	Msg  string
	// Retry is RetrySafe, RetryNever, or "" when neither is known.
	Retry string
}

func (e *Error) Error() string { return "anyprofile:" + e.Code + ": " + e.Msg }

// Safe marks the error as retryable after the step its message names.
func (e *Error) Safe() *Error {
	e.Retry = RetrySafe
	return e
}

// Never marks the error as not retryable as written.
func (e *Error) Never() *Error {
	e.Retry = RetryNever
	return e
}

func New(code, msg string) *Error { return &Error{Code: code, Msg: msg} }

func Newf(code, format string, a ...any) *Error {
	return &Error{Code: code, Msg: fmt.Sprintf(format, a...)}
}

// NewRetryable is New for an error repeating may fix.
func NewRetryable(code, msg string) *Error { return New(code, msg).Safe() }

// NewfRetryable is Newf for an error repeating may fix.
func NewfRetryable(code, format string, a ...any) *Error {
	return Newf(code, format, a...).Safe()
}
