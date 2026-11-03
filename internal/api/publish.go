// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/automation"
	"github.com/D0bby999/anyprofile/internal/publish"
	"github.com/D0bby999/anyprofile/internal/store"
)

// PublishInput is one queued post, as the REST API and the MCP tools take it.
type PublishInput struct {
	ProfileID  string    `json:"profileId" jsonschema:"the profile that will post"`
	Platform   string    `json:"platform" jsonschema:"instagram or facebook"`
	Kind       string    `json:"kind,omitempty" jsonschema:"reel (default) or video"`
	Crop       string    `json:"crop,omitempty" jsonschema:"Instagram crop: Original (default), 1:1, 4:5, 9:16 or 16:9"`
	Title      string    `json:"title,omitempty" jsonschema:"a label for the queue; the platform caption is separate"`
	Caption    string    `json:"caption,omitempty" jsonschema:"the caption or description written on the post"`
	Source     string    `json:"source" jsonschema:"video file name inside the uploads folder"`
	ScheduleAt time.Time `json:"scheduleAt" jsonschema:"when to post, RFC3339 with a zone"`
}

// PublishTasks is the queue the API and MCP expose. The app implements it on
// the service; nil answers 501 and leaves the tools out.
type PublishTasks interface {
	CreatePublishTask(ctx context.Context, in PublishInput) (store.PublishTask, error)
	ListPublishTasks(status string) ([]store.PublishTask, error)
	CancelPublishTask(id string) error
	RetryPublishTask(id string) error
	RunPublishTaskNow(id string) error
}

// RunPublishTask posts one claimed task and returns the URL it read. It starts
// the profile when needed, replays the task's steps through the same executor
// as /v1/act, keeps a screenshot of the failing step, and never deletes the
// source file.
func (s *Server) RunPublishTask(ctx context.Context, t store.PublishTask) (string, error) {
	steps, err := publish.Parse(t.Steps)
	if err != nil {
		return "", err
	}
	if p, err := s.profile(t.ProfileID); err != nil {
		return "", err
	} else if p.State != "running" {
		s.log.Info("publish: starting profile", "profile", p.ID, "task", t.ID)
		if err := s.backend.Start(ctx, p.ID, true); err != nil {
			return "", err
		}
	}
	b, err := s.waitBrowser(ctx, t.ProfileID)
	if err != nil {
		return "", err
	}
	// The task gets its own tab: it must not navigate away from a tab the user
	// is working in (the queue used to reuse whatever tab was in front).
	var target *string
	if len(steps) > 0 && steps[0].Action == "navigate" && steps[0].URL != "" {
		tab, err := b.OpenTab(ctx, steps[0].URL)
		if err != nil {
			return "", err
		}
		target = &tab.ID
		defer func() { _ = b.CloseTab(context.Background(), tab.ID) }()
	}
	for i, st := range steps {
		if target != nil && i == 0 && st.Action == "navigate" {
			continue // the tab already opened at this URL
		}
		// A wait with no text or URL is a plain pause in the template; the
		// executor's wait needs one of them to wait for.
		if st.Action == "wait" && st.Text == "" && st.URL == "" {
			secs := st.Seconds
			if secs <= 0 {
				secs = 1
			}
			if secs > 60 {
				secs = 60
			}
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(secs) * time.Second):
			}
			continue
		}
		in := actRequest{
			Profile: t.ProfileID,
			Action:  st.Action,
			Ref:     st.Ref,
			URL:     st.URL,
			Text:    st.Text,
			Key:     st.Key,
			Files:   st.Files,
			Seconds: st.Seconds,
			Gone:    st.Gone,
		}
		if target != nil {
			in.Tab = *target
		}
		if st.Target != nil {
			in.Target = &automation.Target{Text: st.Target.Text, Role: st.Target.Role,
				Name: st.Target.Name, TestID: st.Target.TestID, Label: st.Target.Label}
		}
		if st.Seconds > 60 {
			in.Seconds = 60
		}
		if _, err := s.doAct(ctx, in); err != nil {
			if st.Optional {
				continue
			}
			dir := s.screenshotStep(ctx, t.ProfileID, t.ID, i, target)
			return "", apperr.Newf("publish_step", "step %d (%s) failed: %s%s", i+1, st.Action, err.Error(), dir)
		}
	}
	return s.publishedURL(ctx, t.ProfileID), nil
}

// screenshotStep saves what the browser showed at the failing step, so the
// queue's error carries a picture of the page that changed.
func (s *Server) screenshotStep(ctx context.Context, profileID, taskID string, step int, tab *string) string {
	if s.PublishArtifacts == "" {
		return ""
	}
	_, b, err := s.browser(profileID)
	if err != nil {
		return ""
	}
	shotTab := ""
	if tab != nil {
		shotTab = *tab
	}
	img, err := b.Screenshot(ctx, shotTab)
	if err != nil {
		return ""
	}
	dir := filepath.Join(s.PublishArtifacts, taskID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	name := filepath.Join(dir, fmt.Sprintf("step-%d.jpg", step+1))
	if err := os.WriteFile(name, img, 0o600); err != nil {
		return ""
	}
	return " (screenshot: " + name + ")"
}

// publishedURL reads the links on the page and returns the newest post URL.
var (
	igPostRe = regexp.MustCompile(`https://www\.instagram\.com/(?:[A-Za-z0-9_.]+/)?(?:p|reel)/[A-Za-z0-9_-]+/?`)
	fbPostRe = regexp.MustCompile(`https://www\.facebook\.com/(?:reel/\d+|share/[^\s"'<>]+|permalink\.php\?story_fbid=\d+)[^\s"'<>]*`)
)

func (s *Server) publishedURL(ctx context.Context, profileID string) string {
	res, err := s.doAct(ctx, actRequest{Profile: profileID, Action: "read", Links: true})
	if err != nil {
		return ""
	}
	text := fmt.Sprint(res)
	for _, re := range []*regexp.Regexp{igPostRe, fbPostRe} {
		if m := re.FindString(text); m != "" {
			return m
		}
	}
	return ""
}

// publishTaskJSON is the REST shape; the store's fields stay unexported names.
type publishTaskJSON struct {
	ID         string `json:"id"`
	ProfileID  string `json:"profileId"`
	Platform   string `json:"platform"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Caption    string `json:"caption"`
	Source     string `json:"source"`
	ScheduleAt string `json:"scheduleAt"`
	Status     string `json:"status"`
	Attempts   int    `json:"attempts"`
	LastError  string `json:"lastError,omitempty"`
	ResultURL  string `json:"resultUrl,omitempty"`
	CreatedAt  string `json:"createdAt"`
}

func publishJSON(t store.PublishTask) publishTaskJSON {
	return publishTaskJSON{ID: t.ID, ProfileID: t.ProfileID, Platform: t.Platform, Kind: t.Kind,
		Title: t.Title, Caption: t.Caption, Source: t.SourcePath, ScheduleAt: t.ScheduleAt,
		Status: t.Status, Attempts: t.Attempts, LastError: t.LastError, ResultURL: t.ResultURL,
		CreatedAt: t.CreatedAt}
}

func (s *Server) requirePublish(w http.ResponseWriter) bool {
	if s.Publish == nil {
		writeErr(w, http.StatusNotImplemented, apperr.New("api_publish", "the publish queue is not available"))
		return false
	}
	return true
}

func (s *Server) publishList(w http.ResponseWriter, r *http.Request) {
	if !s.requirePublish(w) {
		return
	}
	ts, err := s.Publish.ListPublishTasks(r.URL.Query().Get("status"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]publishTaskJSON, 0, len(ts))
	for _, t := range ts {
		out = append(out, publishJSON(t))
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": out})
}

func (s *Server) publishCreate(w http.ResponseWriter, r *http.Request) {
	if !s.requirePublish(w) {
		return
	}
	var in PublishInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, apperr.New("api_body", "the body is not JSON: "+err.Error()))
		return
	}
	t, err := s.Publish.CreatePublishTask(r.Context(), in)
	if err != nil {
		writeErr(w, actStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, publishJSON(t))
}

func (s *Server) publishAction(w http.ResponseWriter, r *http.Request, fn func(string) error) {
	if !s.requirePublish(w) {
		return
	}
	if err := fn(r.PathValue("id")); err != nil {
		writeErr(w, actStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// addPublishTools gives agents the queue: schedule a post, list it, cancel it,
// retry a failed one.
func (s *Server) addPublishTools(srv *mcp.Server) {
	if s.Publish == nil {
		return
	}
	add(s, srv, "publish_schedule", "Queue a video to be posted by AnyProfile at a time, so no agent has to be running then. Instagram and Facebook today; the app posts with the profile's browser and records the link.", false,
		func(ctx context.Context, in PublishInput) (*mcp.CallToolResult, error) {
			t, err := s.Publish.CreatePublishTask(ctx, in)
			if err != nil {
				return nil, err
			}
			return text("queued " + t.ID + " for " + t.ScheduleAt + " on " + t.Platform), nil
		})
	add(s, srv, "publish_list", "The publish queue, newest first: time, platform, profile, status, error and the URL a post got.", true,
		func(_ context.Context, in publishListIn) (*mcp.CallToolResult, error) {
			ts, err := s.Publish.ListPublishTasks(in.Status)
			if err != nil {
				return nil, err
			}
			var sb strings.Builder
			for _, t := range ts {
				fmt.Fprintf(&sb, "%s  %s  %s  %s  %s\n", t.ScheduleAt, t.Platform, t.Status, t.Title, t.ResultURL)
				if t.LastError != "" {
					fmt.Fprintf(&sb, "  error: %s\n", t.LastError)
				}
			}
			if sb.Len() == 0 {
				return text("the queue is empty"), nil
			}
			return text(sb.String()), nil
		})
	add(s, srv, "publish_cancel", "Cancel a queued post that has not run yet.", false,
		func(_ context.Context, in publishIDIn) (*mcp.CallToolResult, error) {
			if err := s.Publish.CancelPublishTask(in.ID); err != nil {
				return nil, err
			}
			return text("cancelled " + in.ID), nil
		})
	add(s, srv, "publish_run", "Run a pending, failed or cancelled post now instead of at its time.", false,
		func(_ context.Context, in publishIDIn) (*mcp.CallToolResult, error) {
			if err := s.Publish.RunPublishTaskNow(in.ID); err != nil {
				return nil, err
			}
			return text("running " + in.ID + " now"), nil
		})
	add(s, srv, "publish_retry", "Put a failed or cancelled post back in the queue, to run now.", false,
		func(_ context.Context, in publishIDIn) (*mcp.CallToolResult, error) {
			if err := s.Publish.RetryPublishTask(in.ID); err != nil {
				return nil, err
			}
			return text("retrying " + in.ID + " now"), nil
		})
}

type publishListIn struct {
	Status string `json:"status,omitempty" jsonschema:"only one status: pending, running, done, failed or cancelled"`
}

func (publishListIn) who() string { return "" }

type publishIDIn struct {
	ID string `json:"id" jsonschema:"task id from publish_list"`
}

func (publishIDIn) who() string { return "" }

func (p PublishInput) who() string { return p.ProfileID }
