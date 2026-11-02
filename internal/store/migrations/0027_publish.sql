-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Publish queue: one row per video to post at a wall-clock time. The steps are
-- the same JSON act steps /v1/run takes, so the queue and the agent drive the
-- browser through one code path. Times are UTC; the UI shows the local zone.
CREATE TABLE publish_tasks (
  id           TEXT PRIMARY KEY,
  profile_id   TEXT NOT NULL,
  platform     TEXT NOT NULL,
  kind         TEXT NOT NULL,
  title        TEXT NOT NULL DEFAULT '',
  caption      TEXT NOT NULL DEFAULT '',
  source_path  TEXT NOT NULL DEFAULT '',
  steps_json   TEXT NOT NULL DEFAULT '[]',
  schedule_at  TEXT NOT NULL,
  status       TEXT NOT NULL DEFAULT 'pending',
  attempts     INTEGER NOT NULL DEFAULT 0,
  last_error   TEXT NOT NULL DEFAULT '',
  result_url   TEXT NOT NULL DEFAULT '',
  artifact_dir TEXT NOT NULL DEFAULT '',
  created_at   TEXT NOT NULL,
  updated_at   TEXT NOT NULL
);
CREATE INDEX publish_due ON publish_tasks(status, schedule_at);
