-- SPDX-License-Identifier: AGPL-3.0-or-later
-- The extension library (unpacked under <data>/extensions/<id>) and which
-- profiles load which extension.
CREATE TABLE extensions (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  version    TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE TABLE profile_extensions (
  profile_id   TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
  extension_id TEXT NOT NULL REFERENCES extensions(id) ON DELETE CASCADE,
  PRIMARY KEY (profile_id, extension_id)
);
