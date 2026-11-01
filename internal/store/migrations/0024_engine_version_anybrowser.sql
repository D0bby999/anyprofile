-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Profiles migrated from the upstream Apostate 0.5.1 build now run the
-- AnyBrowser release that replaced it. 0.5.1 is not an AnyBrowser release,
-- so without this the launcher cannot resolve the installed engine.
UPDATE profiles SET engine_version = '0.5.1.156'
WHERE engine = 'anybrowser' AND engine_version = '0.5.1';
