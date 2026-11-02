// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/store"
)

func TestCreatePublishTaskBuildsSteps(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	s.Clock = func() time.Time { return time.Date(2026, 11, 2, 20, 0, 0, 0, time.UTC) }
	p, err := s.CreateProfile(ctx, NewProfile{Name: "Sigma", Sites: []string{"instagram"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Dirs.Uploads(), "v.mp4"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	task, err := s.CreatePublishTask(ctx, PublishInput{
		ProfileID: p.ID, Platform: "instagram", Kind: "reel", Source: "v.mp4",
		Caption: "hello", ScheduleAt: time.Date(2026, 11, 3, 3, 30, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != "pending" || task.Steps == "" || task.Steps == "[]" {
		t.Fatalf("task = %+v, want pending with steps", task)
	}
	if task.ScheduleAt != "2026-11-03T03:30:00Z" {
		t.Fatalf("schedule = %q, want UTC", task.ScheduleAt)
	}
}

func TestCreatePublishTaskRefusesMissingFileAndPast(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	s.Clock = func() time.Time { return time.Date(2026, 11, 2, 20, 0, 0, 0, time.UTC) }
	p, err := s.CreateProfile(ctx, NewProfile{Name: "Sigma", Sites: []string{"instagram"}})
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 3, 3, 0, 0, 0, time.UTC)
	if _, err := s.CreatePublishTask(ctx, PublishInput{ProfileID: p.ID, Platform: "instagram", Source: "nope.mp4", ScheduleAt: at}); err == nil {
		t.Fatal("a file outside uploads must be refused")
	}
	if _, err := s.CreatePublishTask(ctx, PublishInput{ProfileID: p.ID, Platform: "instagram", Source: "v.mp4", ScheduleAt: time.Date(2026, 11, 1, 3, 0, 0, 0, time.UTC)}); err == nil {
		t.Fatal("a time in the past must be refused")
	}
	if _, err := s.CreatePublishTask(ctx, PublishInput{ProfileID: p.ID, Platform: "tiktok", Source: "v.mp4", ScheduleAt: at}); err == nil {
		t.Fatal("an unsupported platform must be refused")
	}
}

func TestPublishLoopRunsDueOnceAndRecordsURL(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 3, 3, 0, 0, 0, time.UTC)
	s.Clock = func() time.Time { return now }
	p, err := s.CreateProfile(ctx, NewProfile{Name: "Sigma", Sites: []string{"instagram"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Dirs.Uploads(), "v.mp4"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	task, err := s.CreatePublishTask(ctx, PublishInput{ProfileID: p.ID, Platform: "instagram",
		Source: "v.mp4", Caption: "c", ScheduleAt: now})
	if err != nil {
		t.Fatal(err)
	}
	var calls int
	s.SetPublishRunner(func(_ context.Context, t store.PublishTask) (string, error) {
		calls++
		return "https://www.instagram.com/reel/abc/", nil
	})
	s.runDuePublish(ctx)
	s.runDuePublish(ctx) // a second tick must not post again
	if calls != 1 {
		t.Fatalf("runner calls = %d, want 1", calls)
	}
	got, err := s.Store.GetPublishTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "done" || got.ResultURL != "https://www.instagram.com/reel/abc/" {
		t.Fatalf("task = %+v, want done with the URL", got)
	}
}

func TestPublishLoopRetriesThenFails(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 3, 3, 0, 0, 0, time.UTC)
	s.Clock = func() time.Time { return now }
	p, err := s.CreateProfile(ctx, NewProfile{Name: "Sigma", Sites: []string{"instagram"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Dirs.Uploads(), "v.mp4"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	task, err := s.CreatePublishTask(ctx, PublishInput{ProfileID: p.ID, Platform: "instagram",
		Source: "v.mp4", Caption: "c", ScheduleAt: now})
	if err != nil {
		t.Fatal(err)
	}
	s.SetPublishRunner(func(_ context.Context, _ store.PublishTask) (string, error) { return "", os.ErrDeadlineExceeded })
	for i := 0; i < 3; i++ {
		s.runDuePublish(ctx)
		// Move the next retry into the present.
		if t2, err := s.Store.GetPublishTask(task.ID); err == nil && t2.Status == "pending" {
			if err := s.Store.RescheduleTask(t2.ID, now.Format(time.RFC3339), t2.LastError, t2.Attempts); err != nil {
				t.Fatal(err)
			}
		}
	}
	got, err := s.Store.GetPublishTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "failed" || got.Attempts != 3 {
		t.Fatalf("task = %+v, want failed after 3 attempts", got)
	}
}
