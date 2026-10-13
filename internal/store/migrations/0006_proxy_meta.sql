-- SPDX-License-Identifier: AGPL-3.0-or-later
ALTER TABLE proxies ADD COLUMN note TEXT NOT NULL DEFAULT '';
ALTER TABLE proxies ADD COLUMN expires_on TEXT NOT NULL DEFAULT ''; -- YYYY-MM-DD, '' = no date
