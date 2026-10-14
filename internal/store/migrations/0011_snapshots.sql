-- SPDX-License-Identifier: AGPL-3.0-or-later
CREATE TABLE snapshots (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL,
  reason TEXT NOT NULL CHECK (reason IN ('close', 'manual', 'before-restore', 'before-import')),
  bytes INTEGER NOT NULL,
  file TEXT NOT NULL            -- base name inside snapshots/<profile_id>/
);
CREATE INDEX snapshots_profile ON snapshots(profile_id, id DESC);
