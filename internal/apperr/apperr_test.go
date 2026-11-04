// SPDX-License-Identifier: AGPL-3.0-or-later

package apperr

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorFormat(t *testing.T) {
	e := New("x_y", "boom")
	if got := e.Error(); got != "anyprofile:x_y: boom" {
		t.Fatalf("Error() = %q", got)
	}
	if !errors.Is(fmt.Errorf("wrap: %w", e), e) {
		t.Fatal("errors.Is through %w must match")
	}
	if got := Newf("a_b", "n=%d", 3).Msg; got != "n=3" {
		t.Fatalf("Newf msg = %q", got)
	}
}

func TestRetryDisposition(t *testing.T) {
	if got := New("plain", "x").Retry; got != "" {
		t.Fatalf("a plain error says nothing, got %q", got)
	}
	if got := NewRetryable("busy", "x").Retry; got != RetrySafe {
		t.Fatalf("NewRetryable = %q, want %q", got, RetrySafe)
	}
	if got := NewfRetryable("gone", "%d", 1).Retry; got != RetrySafe {
		t.Fatalf("NewfRetryable = %q, want %q", got, RetrySafe)
	}
	if got := New("ambiguous", "x").Never().Retry; got != RetryNever {
		t.Fatalf("Never = %q, want %q", got, RetryNever)
	}
}
