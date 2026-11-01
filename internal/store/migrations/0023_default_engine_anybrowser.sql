-- SPDX-License-Identifier: AGPL-3.0-or-later
-- The saved default engine followed the rename in 0021 for profiles and
-- templates; finish it for settings. A database that still says apostate
-- after schema 21 is post-option data and is rejected before this runs.
UPDATE settings SET value = 'anybrowser' WHERE key = 'default_engine' AND value = 'apostate';
