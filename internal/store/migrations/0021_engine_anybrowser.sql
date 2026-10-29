-- SPDX-License-Identifier: AGPL-3.0-or-later
-- The default engine was renamed from Apostate to AnyBrowser when the
-- project switched from the upstream download to its own fork build
-- (pro/chrome). Rebind existing profiles and templates to the new engine
-- id. Seeds and persona data are unchanged, so no profile's machine moves;
-- the old engine binaries under engines/apostate are left on disk for the
-- user to remove.
UPDATE profiles SET engine = 'anybrowser' WHERE engine = 'apostate';
UPDATE profiles SET persona_id = 'anybrowser-macos' WHERE persona_id = 'apostate-macos';
UPDATE profiles SET persona_id = 'anybrowser-windows' WHERE persona_id = 'apostate-windows';
UPDATE templates SET engine = 'anybrowser' WHERE engine = 'apostate';
