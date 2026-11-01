-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Migration 0024 moved profiles from the upstream Apostate 0.5.1 build
-- (Chromium 155) to AnyBrowser 0.5.1.156 (Chromium 156) but kept the
-- fingerprint baseline measured on the old browser. Every launch after that
-- compared a 155 sample against a 156 one, so it reported drift for the values
-- the update itself moved (the user agent and the canvas hash), launch after
-- launch, with nothing the user had done and no way back to "match".
--
-- AnyBrowser never was Chromium 155, so such a baseline cannot describe what
-- this engine measures now: drop it and let the next launch set a fresh one.
-- The baseline's user agent names the build it was measured on, which is why
-- rows measured on the current build (and profiles of another engine, such as
-- Apostate, which really runs 155) are left untouched.
UPDATE profiles SET fp_baseline = '', fp_last = '', fp_verdict = '', fp_diff = '[]', fp_checked_at = ''
WHERE engine = 'anybrowser' AND fp_baseline LIKE '%Chrome/155.%';
