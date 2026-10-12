-- SPDX-License-Identifier: AGPL-3.0-or-later
-- site replaces channel_kind. channel_kind stays (SQLite cannot drop a
-- column named in a CHECK) but is no longer read or written.
ALTER TABLE profiles ADD COLUMN site TEXT NOT NULL DEFAULT 'other';
UPDATE profiles SET site = channel_kind;
CREATE TABLE custom_sites (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  domains TEXT NOT NULL DEFAULT '[]',  -- JSON array of lowercase host names
  start_url TEXT NOT NULL DEFAULT '',
  color TEXT NOT NULL,
  created_at TEXT NOT NULL
);
