-- SPDX-License-Identifier: AGPL-3.0-or-later
-- The account a profile signs in to. The password and the 2FA secret live in
-- the Keychain; these flags only say whether they are set.
ALTER TABLE profiles ADD COLUMN login TEXT NOT NULL DEFAULT '';
ALTER TABLE profiles ADD COLUMN has_password INTEGER NOT NULL DEFAULT 0;
ALTER TABLE profiles ADD COLUMN has_totp INTEGER NOT NULL DEFAULT 0;
