// SPDX-License-Identifier: AGPL-3.0-or-later

package publish

import "testing"

func TestBuildInstagramReel(t *testing.T) {
	steps, err := Build("instagram", "reel", "Original", "v.mp4", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) < 10 {
		t.Fatalf("steps = %d, want the whole composer flow", len(steps))
	}
	if steps[0].Action != "navigate" || steps[0].URL == "" {
		t.Fatalf("first step = %+v, want a navigate", steps[0])
	}
	var uploaded, shared bool
	for _, s := range steps {
		if s.Action == "upload" && len(s.Files) == 1 && s.Files[0] == "v.mp4" {
			uploaded = true
		}
		if s.Action == "wait" && s.Text == "Shared reel" {
			shared = true
		}
	}
	if !uploaded || !shared {
		t.Fatalf("steps missing upload or confirmation: %+v", steps)
	}
}

func TestBuildFacebookReel(t *testing.T) {
	steps, err := Build("facebook", "reel", "Original", "v.mp4", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if steps[0].URL != "https://www.facebook.com/reels/create/" {
		t.Fatalf("first step = %+v", steps[0])
	}
	var posted bool
	for _, s := range steps {
		if s.Action == "wait" && s.Text == "successfully been shared" {
			posted = true
		}
	}
	if !posted {
		t.Fatalf("steps missing the share confirmation: %+v", steps)
	}
}

func TestBuildRefusesUnknownPlatformAndKind(t *testing.T) {
	if _, err := Build("tiktok", "reel", "Original", "v.mp4", ""); err == nil {
		t.Fatal("tiktok must be refused until it has steps")
	}
	if _, err := Build("instagram", "photo", "Original", "v.mp4", ""); err == nil {
		t.Fatal("kind photo must be refused")
	}
	if _, err := Build("instagram", "reel", "Original", "", ""); err == nil {
		t.Fatal("an empty file must be refused")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	steps, err := Build("instagram", "reel", "Original", "v.mp4", "c")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := JSON(steps)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(back) != len(steps) {
		t.Fatalf("round trip = %d steps, want %d", len(back), len(steps))
	}
	if back[0].Action != "navigate" || back[0].URL != steps[0].URL {
		t.Fatalf("round trip first step = %+v", back[0])
	}
}
