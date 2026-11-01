// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/D0bby999/anyprofile/internal/pro"
)

// TestProLive is opt-in: it talks to a running staging/local Worker and a
// key an admin granted. It proves the app's JWS verifier and device key work
// against the real server, not only the fakes.
func TestProLive(t *testing.T) {
	base := os.Getenv("ANYPROFILE_PRO_TEST_URL")
	key := os.Getenv("ANYPROFILE_PRO_TEST_KEY")
	if base == "" || key == "" {
		t.Skip("set ANYPROFILE_PRO_TEST_URL and ANYPROFILE_PRO_TEST_KEY to run")
	}
	s, _ := newService(t)
	s.Pro = pro.Client{Base: base, HTTP: &http.Client{}}
	ctx := context.Background()

	st, err := s.ProActivate(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Active || st.State != "active" || st.Kind != "lifetime" || len(st.Features) == 0 {
		t.Fatalf("status %+v", st)
	}
	if st.GraceUntil == "" || st.UpdatesThrough == "" {
		t.Fatalf("certificate dates: %+v", st)
	}
	if err := s.mobileAllowed(); err != nil {
		t.Fatalf("mobile gate: %v", err)
	}
	if _, err := s.ProCheck(ctx, true); err != nil {
		t.Fatal(err)
	}
}

func TestProGiftLive(t *testing.T) {
	base := os.Getenv("ANYPROFILE_PRO_TEST_URL")
	code := os.Getenv("ANYPROFILE_PRO_TEST_GIFT")
	if base == "" || code == "" {
		t.Skip("set ANYPROFILE_PRO_TEST_URL and ANYPROFILE_PRO_TEST_GIFT to run")
	}
	s, _ := newService(t)
	s.Pro = pro.Client{Base: base, HTTP: &http.Client{}}
	st, err := s.ProRedeemGift(context.Background(), code)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Active || st.State != "active" || st.Kind != "gift" {
		t.Fatalf("gift status %+v", st)
	}
	if err := s.mobileAllowed(); err != nil {
		t.Fatalf("mobile gate: %v", err)
	}
}
