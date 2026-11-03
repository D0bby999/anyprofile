-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Instagram crops a feed video to the platform default unless the composer's
-- crop step says otherwise; the queue records the ratio it should pick.
ALTER TABLE publish_tasks ADD COLUMN crop TEXT NOT NULL DEFAULT '';
