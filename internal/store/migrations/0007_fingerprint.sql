-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Fingerprint measured at each launch. Not secret; never logged.
ALTER TABLE profiles ADD COLUMN fp_baseline TEXT NOT NULL DEFAULT '';  -- JSON sample
ALTER TABLE profiles ADD COLUMN fp_last TEXT NOT NULL DEFAULT '';      -- JSON sample
ALTER TABLE profiles ADD COLUMN fp_verdict TEXT NOT NULL DEFAULT '';   -- '', baseline, match, drift, unchecked
ALTER TABLE profiles ADD COLUMN fp_diff TEXT NOT NULL DEFAULT '[]';    -- JSON array of field names
ALTER TABLE profiles ADD COLUMN fp_checked_at TEXT NOT NULL DEFAULT '';
