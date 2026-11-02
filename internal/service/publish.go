// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/publish"
	"github.com/D0bby999/anyprofile/internal/store"
)

// PublishInput is what the UI or an agent asks the queue to post.
type PublishInput struct {
	ProfileID  string    `json:"profileId"`
	Platform   string    `json:"platform"`
	Kind       string    `json:"kind"`
	Title      string    `json:"title"`
	Caption    string    `json:"caption"`
	Source     string    `json:"source"`     // file name inside the uploads folder
	ScheduleAt time.Time `json:"scheduleAt"` // local time from the caller
}

// PublishRunner posts one claimed task and returns the public URL it read.
// The app wires the browser runner; nil leaves tasks pending and says so.
type PublishRunner func(ctx context.Context, t store.PublishTask) (string, error)

// PublishRunner is set by the app once its browser driver exists.
func (s *Service) SetPublishRunner(r PublishRunner) { s.publishRunner = r }

// StartPublishLoop recovers tasks a crash left running, then ticks until ctx
// ends. A task runs at most once per claim; failed attempts are retried with a
// growing gap, then failed for good.
func (s *Service) StartPublishLoop(ctx context.Context) {
	if n, err := s.Store.RecoverRunningTasks(); err != nil {
		s.Log.Warn("publish: recover running tasks", "err", err)
	} else if n > 0 {
		s.Log.Info("publish: marked interrupted tasks failed", "count", n)
	}
	go func() {
		t := time.NewTicker(20 * time.Second)
		defer t.Stop()
		s.runDuePublish(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.runDuePublish(ctx)
			}
		}
	}()
}

func (s *Service) runDuePublish(ctx context.Context) {
	now := s.now().UTC()
	tasks, err := s.Store.ClaimDueTasks(now.Format(time.RFC3339), 5)
	if err != nil {
		s.Log.Warn("publish: claim due tasks", "err", err)
		return
	}
	for _, t := range tasks {
		if s.publishRunner == nil {
			_ = s.Store.RescheduleTask(t.ID, s.now().Add(time.Minute).UTC().Format(time.RFC3339),
				"the publish runner is not ready (is the local API on?)", t.Attempts)
			continue
		}
		if t.Steps == "" || t.Steps == "[]" {
			_ = s.Store.FinishPublishTask(t.ID, "failed", "", "the task has no steps", "", t.Attempts)
			s.notify("publish")
			continue
		}
		runCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
		url, err := s.publishRunner(runCtx, t)
		cancel()
		attempts := t.Attempts + 1
		if err != nil {
			s.Log.Warn("publish: task failed", "id", t.ID, "platform", t.Platform, "attempt", attempts, "err", err)
			if attempts >= 3 {
				_ = s.Store.FinishPublishTask(t.ID, "failed", "", err.Error(), "", attempts)
			} else {
				_ = s.Store.RescheduleTask(t.ID, publish.Until(s.now(), attempts).UTC().Format(time.RFC3339), err.Error(), attempts)
			}
			s.notify("publish")
			continue
		}
		_ = s.Store.FinishPublishTask(t.ID, "done", url, "", "", attempts)
		s.Log.Info("publish: task done", "id", t.ID, "platform", t.Platform, "url", url)
		s.notify("publish")
	}
}

// CreatePublishTask validates the input, builds the platform steps and stores
// the task. The file must already be in the uploads folder: the queue never
// moves or deletes it.
func (s *Service) CreatePublishTask(ctx context.Context, in PublishInput) (store.PublishTask, error) {
	_ = ctx
	if _, err := s.Store.GetProfile(in.ProfileID); err != nil {
		return store.PublishTask{}, apperr.New("publish_profile", "that profile does not exist")
	}
	if !publish.ValidPlatform(in.Platform) {
		return store.PublishTask{}, apperr.Newf("publish_platform", "the queue cannot post to %q yet", in.Platform)
	}
	if in.Kind == "" {
		in.Kind = "reel"
	}
	name := filepath.Base(strings.TrimSpace(in.Source))
	if name == "" || name == "." || name == ".." {
		return store.PublishTask{}, apperr.New("publish_source", "give the video file name in the uploads folder")
	}
	if _, err := os.Stat(filepath.Join(s.Dirs.Uploads(), name)); err != nil {
		return store.PublishTask{}, apperr.Newf("publish_source", "%s is not in the uploads folder", name)
	}
	if in.ScheduleAt.IsZero() || !in.ScheduleAt.After(s.now().Add(-time.Minute)) {
		return store.PublishTask{}, apperr.New("publish_time", "give a time in the future")
	}
	steps, err := publish.Build(in.Platform, in.Kind, name, in.Caption)
	if err != nil {
		return store.PublishTask{}, err
	}
	raw, err := publish.JSON(steps)
	if err != nil {
		return store.PublishTask{}, err
	}
	t := store.PublishTask{
		ID:         fmt.Sprintf("%s-%s-%d", in.Platform, randomHex(4), s.now().UnixNano()),
		ProfileID:  in.ProfileID,
		Platform:   in.Platform,
		Kind:       in.Kind,
		Title:      in.Title,
		Caption:    in.Caption,
		SourcePath: name,
		Steps:      raw,
		ScheduleAt: in.ScheduleAt.UTC().Format(time.RFC3339),
		Status:     "pending",
	}
	if err := s.Store.CreatePublishTask(&t); err != nil {
		return store.PublishTask{}, err
	}
	s.Log.Info("publish: task queued", "id", t.ID, "platform", t.Platform, "at", t.ScheduleAt)
	s.notify("publish")
	return t, nil
}

func (s *Service) ListPublishTasks(status string) ([]store.PublishTask, error) {
	return s.Store.ListPublishTasks(status, 200)
}

func (s *Service) CancelPublishTask(id string) error {
	if err := s.Store.CancelPublishTask(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return apperr.New("publish_cancel", "only a pending task can be cancelled")
		}
		return err
	}
	s.notify("publish")
	return nil
}

func (s *Service) RetryPublishTask(id string) error {
	if err := s.Store.RetryPublishTask(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return apperr.New("publish_retry", "only a failed or cancelled task can be retried")
		}
		return err
	}
	s.notify("publish")
	return nil
}
