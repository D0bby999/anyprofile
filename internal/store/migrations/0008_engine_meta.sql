-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Values an engine pins for the profile at creation (JSON object).
ALTER TABLE profiles ADD COLUMN engine_meta TEXT NOT NULL DEFAULT '{}';
