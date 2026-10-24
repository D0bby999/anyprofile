-- SPDX-License-Identifier: AGPL-3.0-or-later
-- When the last check of a proxy failed; '' once a check passes. The exit
-- found by the last good check stays, so profiles keep their timezone.
ALTER TABLE proxies ADD COLUMN failed_at TEXT NOT NULL DEFAULT '';
