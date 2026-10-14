-- SPDX-License-Identifier: AGPL-3.0-or-later
-- A profile can belong to several platforms (one account each). "site" keeps
-- the first one, so an older build still reads a sensible platform.
ALTER TABLE profiles ADD COLUMN sites TEXT NOT NULL DEFAULT '[]';
UPDATE profiles SET sites = json_array(site);
