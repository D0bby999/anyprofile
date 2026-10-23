-- SPDX-License-Identifier: AGPL-3.0-or-later
-- A mobile device is an Android machine: a QEMU VM on macOS, an LDPlayer
-- instance on Windows. It shares proxies with profiles and nothing else.
CREATE TABLE mobile_devices (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  driver TEXT NOT NULL CHECK (driver IN ('vm', 'ldplayer')),
  platform TEXT NOT NULL,              -- the OS that created it, the only one that opens it
  driver_ref TEXT NOT NULL DEFAULT '', -- vm: folder name; ldplayer: instance name, never its index
  proxy_id TEXT REFERENCES proxies(id) ON DELETE SET NULL,
  ram_mb INTEGER NOT NULL,
  cpus INTEGER NOT NULL,
  image_version TEXT NOT NULL DEFAULT '',
  notes TEXT NOT NULL DEFAULT '',
  tags TEXT NOT NULL DEFAULT '[]',     -- JSON array of strings
  pinned INTEGER NOT NULL DEFAULT 0,
  sort_order INTEGER NOT NULL DEFAULT 0,
  serial INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  last_open_at TEXT,
  backed_up_at TEXT,
  deleted_at TEXT
);
CREATE INDEX mobile_devices_deleted ON mobile_devices(deleted_at);
CREATE UNIQUE INDEX mobile_devices_serial ON mobile_devices(serial);

-- A running device, so the app can adopt it after a restart.
CREATE TABLE mobile_launches (
  device_id TEXT PRIMARY KEY REFERENCES mobile_devices(id) ON DELETE CASCADE,
  pid INTEGER NOT NULL,
  pid_start_time TEXT NOT NULL,
  adb_port INTEGER NOT NULL DEFAULT 0,
  relay_port INTEGER NOT NULL DEFAULT 0,
  started_at TEXT NOT NULL
);
