-- SPDX-License-Identifier: AGPL-3.0-or-later
-- pragma:no-foreign-keys
-- Allow AnyMobile Metal ('avo') and MuMuPlayer 12 ('mumu12') in mobile_devices.driver,
-- and migrate existing macOS ('darwin') devices from legacy QEMU ('vm') to 'avo'.
CREATE TABLE mobile_devices_new (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  driver TEXT NOT NULL CHECK (driver IN ('avo', 'vm', 'ldplayer', 'mumu12')),
  platform TEXT NOT NULL,
  driver_ref TEXT NOT NULL DEFAULT '',
  proxy_id TEXT REFERENCES proxies(id) ON DELETE SET NULL,
  ram_mb INTEGER NOT NULL,
  cpus INTEGER NOT NULL,
  image_version TEXT NOT NULL DEFAULT '',
  notes TEXT NOT NULL DEFAULT '',
  tags TEXT NOT NULL DEFAULT '[]',
  pinned INTEGER NOT NULL DEFAULT 0,
  sort_order INTEGER NOT NULL DEFAULT 0,
  serial INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  last_open_at TEXT,
  backed_up_at TEXT,
  deleted_at TEXT,
  preset TEXT NOT NULL DEFAULT 'balanced',
  screen_w INTEGER NOT NULL DEFAULT 720,
  screen_h INTEGER NOT NULL DEFAULT 1600,
  density INTEGER NOT NULL DEFAULT 280,
  with_gms INTEGER NOT NULL DEFAULT 0,
  identity_json TEXT NOT NULL DEFAULT ''
);

INSERT INTO mobile_devices_new (
  id, name, driver, platform, driver_ref, proxy_id, ram_mb, cpus,
  image_version, notes, tags, pinned, sort_order, serial,
  created_at, updated_at, last_open_at, backed_up_at, deleted_at,
  preset, screen_w, screen_h, density, with_gms, identity_json
)
SELECT
  id, name,
  CASE WHEN platform = 'darwin' AND driver = 'vm' THEN 'avo' ELSE driver END,
  platform, driver_ref, proxy_id, ram_mb, cpus,
  image_version, notes, tags, pinned, sort_order, serial,
  created_at, updated_at, last_open_at, backed_up_at, deleted_at,
  preset, screen_w, screen_h, density, with_gms, identity_json
FROM mobile_devices;

DROP TABLE mobile_devices;
ALTER TABLE mobile_devices_new RENAME TO mobile_devices;
CREATE INDEX mobile_devices_deleted ON mobile_devices(deleted_at);
CREATE UNIQUE INDEX mobile_devices_serial ON mobile_devices(serial);
