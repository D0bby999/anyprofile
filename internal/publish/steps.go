// SPDX-License-Identifier: AGPL-3.0-or-later

// Package publish builds the act steps the queue replays for one platform.
// A step is the same JSON shape /v1/run takes, so the runner and the agent use
// one executor. The targets are accessible names measured in the browser on
// 2026-11-02; platforms change their UI, so a failed step keeps the screenshot
// and the queue can be retried with a new template version.
package publish

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

// Target matches automation.Target's JSON: text, role, name, testId, label.
type Target struct {
	Text   string `json:"text,omitempty"`
	Role   string `json:"role,omitempty"`
	Name   string `json:"name,omitempty"`
	TestID string `json:"testId,omitempty"`
	Label  string `json:"label,omitempty"`
}

// Step is one act request. Optional steps (a notice that may not appear) are
// skipped when they fail instead of failing the whole task.
type Step struct {
	Action   string   `json:"action"`
	Target   *Target  `json:"target,omitempty"`
	Ref      string   `json:"ref,omitempty"`
	URL      string   `json:"url,omitempty"`
	Text     string   `json:"text,omitempty"`
	Key      string   `json:"key,omitempty"`
	Files    []string `json:"files,omitempty"`
	Seconds  int      `json:"seconds,omitempty"`
	Gone     bool     `json:"gone,omitempty"`
	Optional bool     `json:"optional,omitempty"`
}

// TemplateVersion changes when a platform's steps change; a task records the
// version it was built with so a failure can be traced to a UI change.
const TemplateVersion = 1

// Platforms the queue can drive today.
var Platforms = []string{"instagram", "facebook"}

// ValidPlatform reports whether p is one the templates cover.
func ValidPlatform(p string) bool {
	for _, x := range Platforms {
		if x == p {
			return true
		}
	}
	return false
}

// Build returns the steps for one post: navigate, open the composer, upload the
// file, write the caption, publish, and wait for the platform's confirmation.
func Build(platform, kind string, source string, caption string) ([]Step, error) {
	if !ValidPlatform(platform) {
		return nil, apperr.Newf("publish_platform", "the queue cannot post to %q yet", platform)
	}
	if kind != "reel" && kind != "video" {
		return nil, apperr.Newf("publish_kind", "the queue does not know how to post a %q", kind)
	}
	if strings.TrimSpace(source) == "" {
		return nil, apperr.New("publish_source", "give the video file name in the uploads folder")
	}
	switch platform {
	case "instagram":
		return instagram(kind, source, caption), nil
	case "facebook":
		return facebook(kind, source, caption), nil
	}
	return nil, apperr.Newf("publish_platform", "no steps for %q", platform)
}

// instagram follows the web composer: Create -> Post -> Select from computer
// -> OK notice (optional) -> Next (crop) -> Next (edit) -> caption -> Share.
// A vertical file becomes a Reel by itself; that notice is the optional OK.
func instagram(kind, source, caption string) []Step {
	return []Step{
		{Action: "navigate", URL: "https://www.instagram.com/"},
		{Action: "wait", Seconds: 4},
		{Action: "click", Target: &Target{Role: "link", Name: "New post"}},
		{Action: "wait", Seconds: 2},
		{Action: "click", Target: &Target{Role: "link", Name: "Post Post"}},
		{Action: "wait", Seconds: 2},
		{Action: "upload", Target: &Target{Role: "button", Name: "Select from computer"}, Files: []string{source}},
		{Action: "wait", Seconds: 8},
		{Action: "click", Target: &Target{Role: "button", Name: "OK"}, Optional: true},
		{Action: "wait", Seconds: 2},
		{Action: "click", Target: &Target{Role: "button", Name: "Next"}},
		{Action: "wait", Seconds: 3},
		{Action: "click", Target: &Target{Role: "button", Name: "Next"}},
		{Action: "wait", Seconds: 3},
		{Action: "click", Target: &Target{Role: "textbox", Name: "Add a caption..."}},
		{Action: "type", Target: &Target{Role: "textbox", Name: "Add a caption..."}, Text: caption},
		{Action: "wait", Seconds: 1},
		{Action: "click", Target: &Target{Role: "button", Name: "Share"}},
		{Action: "wait", Text: "Shared reel", Seconds: 60},
	}
}

// facebook follows /reels/create: Add video -> caption -> Next -> Post.
func facebook(kind, source, caption string) []Step {
	return []Step{
		{Action: "navigate", URL: "https://www.facebook.com/reels/create/"},
		{Action: "wait", Seconds: 6},
		{Action: "upload", Target: &Target{Role: "button", Name: "Add video or drag and drop"}, Files: []string{source}},
		{Action: "wait", Seconds: 15},
		{Action: "click", Target: &Target{Role: "textbox", Name: "Describe your reel..."}},
		{Action: "type", Target: &Target{Role: "textbox", Name: "Describe your reel..."}, Text: caption},
		{Action: "wait", Seconds: 1},
		{Action: "click", Target: &Target{Role: "button", Name: "Next"}},
		{Action: "wait", Seconds: 3},
		{Action: "click", Target: &Target{Role: "button", Name: "Post"}},
		{Action: "wait", Text: "successfully been shared", Seconds: 60},
	}
}

// JSON marshals the steps for storing in the task.
func JSON(steps []Step) (string, error) {
	b, err := json.Marshal(steps)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Parse is the runner's side.
func Parse(s string) ([]Step, error) {
	var out []Step
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("steps are not valid JSON: %w", err)
	}
	return out, nil
}

// Until formats the next retry time for a failed attempt.
func Until(now time.Time, attempt int) time.Time {
	switch {
	case attempt <= 1:
		return now.Add(5 * time.Minute)
	case attempt == 2:
		return now.Add(15 * time.Minute)
	default:
		return now.Add(60 * time.Minute)
	}
}
