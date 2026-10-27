-- SPDX-License-Identifier: AGPL-3.0-or-later
-- User profile templates: reusable creation defaults per platform.
-- Built-in templates live in code, not in this table. A template never
-- holds a secret, a concrete proxy or a cookie — the proxy column is a
-- display recommendation only.
CREATE TABLE templates (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  site       TEXT NOT NULL DEFAULT '',
  sites      TEXT NOT NULL DEFAULT '[]',
  proxy_note TEXT NOT NULL DEFAULT '',
  languages  TEXT NOT NULL DEFAULT '',
  ui_locale  TEXT NOT NULL DEFAULT '',
  engine     TEXT NOT NULL DEFAULT '',
  start_url  TEXT NOT NULL DEFAULT '',
  notes      TEXT NOT NULL DEFAULT '',
  tags       TEXT NOT NULL DEFAULT '[]',
  folder_id  TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
