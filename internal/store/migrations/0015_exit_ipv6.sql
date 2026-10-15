-- SPDX-License-Identifier: AGPL-3.0-or-later
-- A proxy can leave through an IPv6 address somewhere else than its IPv4
-- one; sites with IPv6 see that one. Its address and timezone, when it differs.
ALTER TABLE proxies ADD COLUMN exit_ip6 TEXT NOT NULL DEFAULT '';
ALTER TABLE proxies ADD COLUMN timezone6 TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN exit_ip6 TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN exit_tz6 TEXT NOT NULL DEFAULT '';
