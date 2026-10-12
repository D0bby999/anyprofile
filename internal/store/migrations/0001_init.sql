-- SPDX-License-Identifier: AGPL-3.0-or-later
CREATE TABLE proxies (
  id TEXT PRIMARY KEY,
  label TEXT NOT NULL,
  scheme TEXT NOT NULL CHECK (scheme IN ('http', 'socks5')),
  host TEXT NOT NULL,
  port INTEGER NOT NULL,
  username TEXT NOT NULL DEFAULT '',   -- the password lives in the Keychain, account = id
  exit_ip TEXT, country TEXT, timezone TEXT, lat REAL, lon REAL,
  rotating INTEGER NOT NULL DEFAULT 0, -- exit IP changed between probes
  checked_at TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE profiles (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  engine TEXT NOT NULL,
  engine_version TEXT NOT NULL,        -- core version at creation, to warn on change
  persona_id TEXT NOT NULL,
  seed_hex TEXT NOT NULL,
  platform TEXT NOT NULL,
  proxy_id TEXT REFERENCES proxies(id) ON DELETE SET NULL,
  timezone TEXT NOT NULL,
  languages TEXT NOT NULL,             -- comma separated, e.g. "en-US,en"
  ui_locale TEXT NOT NULL,
  lat REAL, lon REAL,
  channel_kind TEXT NOT NULL DEFAULT 'other' CHECK (channel_kind IN ('tiktok', 'youtube', 'other')),
  start_url TEXT NOT NULL DEFAULT '',
  notes TEXT NOT NULL DEFAULT '',
  tags TEXT NOT NULL DEFAULT '[]',     -- JSON array of strings
  cdp_next_launch INTEGER NOT NULL DEFAULT 0,
  pinned INTEGER NOT NULL DEFAULT 0,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  last_open_at TEXT,
  deleted_at TEXT
);
CREATE INDEX profiles_deleted ON profiles(deleted_at);

CREATE TABLE launches (
  profile_id TEXT PRIMARY KEY REFERENCES profiles(id) ON DELETE CASCADE,
  pid INTEGER NOT NULL,
  pid_start_time TEXT NOT NULL,
  relay_port INTEGER NOT NULL DEFAULT 0,
  cdp_port INTEGER NOT NULL DEFAULT 0,
  started_at TEXT NOT NULL
);
