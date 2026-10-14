-- SPDX-License-Identifier: AGPL-3.0-or-later
-- One row per browser launch. exit_* is measured through the profile's proxy
-- right after the launch; empty for profiles without a proxy.
CREATE TABLE sessions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
  engine TEXT NOT NULL,
  engine_version TEXT NOT NULL,
  proxy_id TEXT NOT NULL DEFAULT '',
  started_at TEXT NOT NULL,
  stopped_at TEXT NOT NULL DEFAULT '',
  exit_ip TEXT NOT NULL DEFAULT '',
  exit_country TEXT NOT NULL DEFAULT '',
  exit_tz TEXT NOT NULL DEFAULT '',
  exit_error TEXT NOT NULL DEFAULT '',
  checks TEXT NOT NULL DEFAULT '[]'   -- filled by the consistency checks
);
CREATE INDEX sessions_profile ON sessions(profile_id, id DESC);
