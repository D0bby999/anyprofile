-- SPDX-License-Identifier: AGPL-3.0-or-later
-- One account per platform of a profile. Passwords and 2FA secrets stay in
-- the Keychain under secret_key; accounts saved before this keep their old
-- key (the profile id), so nothing moves in the Keychain. The login columns
-- on profiles are no longer written.
CREATE TABLE accounts (
  profile_id   TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
  site         TEXT NOT NULL,
  login        TEXT NOT NULL DEFAULT '',
  has_password INTEGER NOT NULL DEFAULT 0,
  has_totp     INTEGER NOT NULL DEFAULT 0,
  secret_key   TEXT NOT NULL,
  PRIMARY KEY (profile_id, site)
);
INSERT INTO accounts (profile_id, site, login, has_password, has_totp, secret_key)
  SELECT id, site, login, has_password, has_totp, id FROM profiles
  WHERE login <> '' OR has_password = 1 OR has_totp = 1;
