// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"database/sql"
	"errors"
	"time"
)

// PublishTask is one video to post at a wall-clock time. Steps is the JSON act
// step list the runner replays; ScheduleAt is UTC. The queue never runs a task
// twice: Claim moves pending -> running in one transaction.
type PublishTask struct {
	ID          string `json:"id"`
	ProfileID   string `json:"profileId"`
	Platform    string `json:"platform"` // instagram | facebook | youtube
	Kind        string `json:"kind"`     // reel | video | short
	Title       string `json:"title"`
	Caption     string `json:"caption"`
	SourcePath  string `json:"source"`     // file name inside the uploads folder
	Steps       string `json:"-"`          // JSON act steps; the UI does not need them
	ScheduleAt  string `json:"scheduleAt"` // RFC3339 UTC
	Status      string `json:"status"`     // pending | running | done | failed | cancelled
	Attempts    int    `json:"attempts"`
	LastError   string `json:"lastError"`
	ResultURL   string `json:"resultUrl"`
	ArtifactDir string `json:"artifactDir"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

func (s *Store) CreatePublishTask(t *PublishTask) error {
	now := now()
	if t.CreatedAt == "" {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	_, err := s.db.Exec(`INSERT INTO publish_tasks
		(id, profile_id, platform, kind, title, caption, source_path, steps_json, schedule_at,
		 status, attempts, last_error, result_url, artifact_dir, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.ProfileID, t.Platform, t.Kind, t.Title, t.Caption, t.SourcePath, t.Steps, t.ScheduleAt,
		t.Status, t.Attempts, t.LastError, t.ResultURL, t.ArtifactDir, t.CreatedAt, t.UpdatedAt)
	return err
}

func (s *Store) GetPublishTask(id string) (PublishTask, error) {
	var t PublishTask
	err := s.db.QueryRow(`SELECT id, profile_id, platform, kind, title, caption, source_path, steps_json,
		schedule_at, status, attempts, last_error, result_url, artifact_dir, created_at, updated_at
		FROM publish_tasks WHERE id = ?`, id).Scan(&t.ID, &t.ProfileID, &t.Platform, &t.Kind, &t.Title,
		&t.Caption, &t.SourcePath, &t.Steps, &t.ScheduleAt, &t.Status, &t.Attempts, &t.LastError,
		&t.ResultURL, &t.ArtifactDir, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return PublishTask{}, ErrNotFound
	}
	return t, err
}

// ListPublishTasks returns newest first, optionally only one status.
func (s *Store) ListPublishTasks(status string, limit int) ([]PublishTask, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT id, profile_id, platform, kind, title, caption, source_path, steps_json,
		schedule_at, status, attempts, last_error, result_url, artifact_dir, created_at, updated_at
		FROM publish_tasks`
	args := []any{}
	if status != "" {
		q += ` WHERE status = ?`
		args = append(args, status)
	}
	q += ` ORDER BY schedule_at DESC, created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PublishTask
	for rows.Next() {
		var t PublishTask
		if err := rows.Scan(&t.ID, &t.ProfileID, &t.Platform, &t.Kind, &t.Title, &t.Caption, &t.SourcePath,
			&t.Steps, &t.ScheduleAt, &t.Status, &t.Attempts, &t.LastError, &t.ResultURL, &t.ArtifactDir,
			&t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ClaimDueTasks moves every pending task whose time has come to running and
// returns them. The UPDATE ... RETURNING keeps a task in exactly one claim even
// if two loops tick together.
func (s *Store) ClaimDueTasks(now string, limit int) ([]PublishTask, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(`UPDATE publish_tasks SET status = 'running', updated_at = ?
		WHERE id IN (SELECT id FROM publish_tasks WHERE status = 'pending' AND schedule_at <= ?
			ORDER BY schedule_at LIMIT ?)
		RETURNING id, profile_id, platform, kind, title, caption, source_path, steps_json,
		schedule_at, status, attempts, last_error, result_url, artifact_dir, created_at, updated_at`,
		now, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PublishTask
	for rows.Next() {
		var t PublishTask
		if err := rows.Scan(&t.ID, &t.ProfileID, &t.Platform, &t.Kind, &t.Title, &t.Caption, &t.SourcePath,
			&t.Steps, &t.ScheduleAt, &t.Status, &t.Attempts, &t.LastError, &t.ResultURL, &t.ArtifactDir,
			&t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// CancelPublishTask cancels a pending task; a running one is left alone.
func (s *Store) CancelPublishTask(id string) error {
	res, err := s.db.Exec(`UPDATE publish_tasks SET status = 'cancelled', updated_at = ?
		WHERE id = ? AND status = 'pending'`, now(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RetryPublishTask puts a failed or cancelled task back in the queue now.
func (s *Store) RetryPublishTask(id string) error {
	res, err := s.db.Exec(`UPDATE publish_tasks SET status = 'pending', schedule_at = ?, last_error = '', updated_at = ?
		WHERE id = ? AND status IN ('failed','cancelled')`, now(), now(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// FinishPublishTask records the outcome. A failure keeps the task pending for a
// retry until attempts reach the limit; the caller decides by passing status.
func (s *Store) FinishPublishTask(id, status, resultURL, lastError, artifactDir string, attempts int) error {
	_, err := s.db.Exec(`UPDATE publish_tasks SET status = ?, result_url = ?, last_error = ?,
		artifact_dir = ?, attempts = ?, updated_at = ? WHERE id = ?`,
		status, resultURL, lastError, artifactDir, attempts, now(), id)
	return err
}

// RecoverRunningTasks marks tasks a crash left running as failed, so the queue
// does not believe a post is in flight forever.
func (s *Store) RecoverRunningTasks() (int, error) {
	res, err := s.db.Exec(`UPDATE publish_tasks SET status = 'failed',
		last_error = 'the app closed while this task was running', updated_at = ? WHERE status = 'running'`, now())
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// NowUTC is the queue's clock in the stored format; tests replace it with a
// fixed time by calling ClaimDueTasks directly.
func NowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

// RescheduleTask puts a failed attempt back in the queue at a later time.
func (s *Store) RescheduleTask(id, at, lastError string, attempts int) error {
	_, err := s.db.Exec(`UPDATE publish_tasks SET status = 'pending', schedule_at = ?, last_error = ?,
		attempts = ?, updated_at = ? WHERE id = ?`, at, lastError, attempts, now(), id)
	return err
}

// RunPublishTaskNow puts a task that has not run yet (pending, failed or
// cancelled) back in the queue at the current minute.
func (s *Store) RunPublishTaskNow(id string) error {
	res, err := s.db.Exec(`UPDATE publish_tasks SET status = 'pending', schedule_at = ?, last_error = '',
		updated_at = ? WHERE id = ? AND status IN ('pending','failed','cancelled')`, now(), now(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdatePublishTaskSteps replaces the stored steps, so a retry after a template
// fix replays the current one instead of the one that failed.
func (s *Store) UpdatePublishTaskSteps(id, steps string) error {
	_, err := s.db.Exec(`UPDATE publish_tasks SET steps_json = ?, updated_at = ? WHERE id = ?`, steps, now(), id)
	return err
}
