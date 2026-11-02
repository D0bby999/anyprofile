// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import "testing"

func seedPubStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(dir+"/pub.db", dir+"/backups")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	mkProfile(t, s, "p1")
	return s
}

func newPublishTask(t *testing.T, s *Store, id, at string) PublishTask {
	t.Helper()
	tsk := PublishTask{ID: id, ProfileID: "p1", Platform: "instagram", Kind: "reel",
		Title: "t", Caption: "c", SourcePath: "v.mp4", Steps: "[]", ScheduleAt: at, Status: "pending"}
	if err := s.CreatePublishTask(&tsk); err != nil {
		t.Fatal(err)
	}
	return tsk
}

func TestClaimDueRunsEachTaskOnce(t *testing.T) {
	s := seedPubStore(t)
	newPublishTask(t, s, "a", "2026-11-03T03:00:00Z")
	newPublishTask(t, s, "b", "2026-11-05T03:00:00Z") // not due yet

	got, err := s.ClaimDueTasks("2026-11-03T03:00:01Z", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "a" || got[0].Status != "running" {
		t.Fatalf("claim = %+v, want only a running", got)
	}
	again, err := s.ClaimDueTasks("2026-11-03T03:00:02Z", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("second claim = %+v, want none", again)
	}
}

func TestCancelOnlyPending(t *testing.T) {
	s := seedPubStore(t)
	newPublishTask(t, s, "a", "2026-11-03T03:00:00Z")
	if err := s.CancelPublishTask("a"); err != nil {
		t.Fatal(err)
	}
	if err := s.CancelPublishTask("a"); err == nil {
		t.Fatal("cancelling a cancelled task should fail")
	}
	if _, err := s.ClaimDueTasks("2026-11-03T03:00:01Z", 10); err != nil {
		t.Fatal(err)
	}
}

func TestRecoverRunningMarksCrashedTasks(t *testing.T) {
	s := seedPubStore(t)
	newPublishTask(t, s, "a", "2026-11-03T03:00:00Z")
	if _, err := s.ClaimDueTasks("2026-11-03T03:00:01Z", 10); err != nil {
		t.Fatal(err)
	}
	n, err := s.RecoverRunningTasks()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("recovered %d, want 1", n)
	}
	got, err := s.GetPublishTask("a")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "failed" || got.LastError == "" {
		t.Fatalf("task after recovery = %+v, want failed with a reason", got)
	}
}

func TestRetryPutsFailedBackNow(t *testing.T) {
	s := seedPubStore(t)
	newPublishTask(t, s, "a", "2026-11-03T03:00:00Z")
	if err := s.FinishPublishTask("a", "failed", "", "boom", "", 1); err != nil {
		t.Fatal(err)
	}
	if err := s.RetryPublishTask("a"); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetPublishTask("a")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "pending" || got.LastError != "" {
		t.Fatalf("task after retry = %+v, want pending without error", got)
	}
}
