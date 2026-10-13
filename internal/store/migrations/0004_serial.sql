-- SPDX-License-Identifier: AGPL-3.0-or-later
-- serial is the profile's number in the table and in its window title.
-- Assigned once at creation, never reused while the profile exists.
ALTER TABLE profiles ADD COLUMN serial INTEGER NOT NULL DEFAULT 0;
UPDATE profiles SET serial = (
  SELECT n FROM (SELECT id, ROW_NUMBER() OVER (ORDER BY created_at, id) AS n FROM profiles) r
  WHERE r.id = profiles.id
);
CREATE UNIQUE INDEX profiles_serial ON profiles(serial);
