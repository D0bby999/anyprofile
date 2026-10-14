-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Default labels used to be the redacted URL, which shows the username.
UPDATE proxies SET label = host || ':' || port WHERE label LIKE '%@%';
