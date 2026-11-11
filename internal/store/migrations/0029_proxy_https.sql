-- SPDX-License-Identifier: AGPL-3.0-or-later
-- pragma:no-foreign-keys
-- Allow https upstream proxies in the proxies table CHECK constraint.
CREATE TABLE proxies_new (
  id TEXT PRIMARY KEY,
  label TEXT NOT NULL,
  scheme TEXT NOT NULL CHECK (scheme IN ('http', 'https', 'socks5')),
  host TEXT NOT NULL,
  port INTEGER NOT NULL,
  username TEXT NOT NULL DEFAULT '',
  exit_ip TEXT,
  country TEXT,
  timezone TEXT,
  lat REAL,
  lon REAL,
  rotating INTEGER NOT NULL DEFAULT 0,
  checked_at TEXT,
  created_at TEXT NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  expires_on TEXT NOT NULL DEFAULT '',
  exit_ip6 TEXT NOT NULL DEFAULT '',
  timezone6 TEXT NOT NULL DEFAULT '',
  failed_at TEXT NOT NULL DEFAULT ''
);

INSERT INTO proxies_new (
  id, label, scheme, host, port, username, exit_ip, country, timezone,
  lat, lon, rotating, checked_at, created_at, note, expires_on, exit_ip6, timezone6, failed_at
)
SELECT
  id, label, scheme, host, port, username, exit_ip, country, timezone,
  lat, lon, rotating, checked_at, created_at, note, expires_on, exit_ip6, timezone6, failed_at
FROM proxies;

DROP TABLE proxies;
ALTER TABLE proxies_new RENAME TO proxies;

