-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Folders group profiles by channel, client or purpose: one profile in
-- at most one folder, NULL means unfiled. Deleting a folder never
-- deletes profiles; the service moves them first (or unfiles them).
CREATE TABLE folders (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  color      TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL
);
-- Name uniqueness (trimmed, case-insensitive, Unicode-aware) is enforced
-- by the store in Go: SQLite's lower() folds ASCII only, which would let
-- "Khách A" and "KHÁCH A" coexist.
ALTER TABLE profiles ADD COLUMN folder_id TEXT REFERENCES folders(id);
CREATE INDEX profiles_folder ON profiles(folder_id);
