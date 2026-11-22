-- Per-APK hardware fingerprint (IdentityJSON) and Android SSAID (android_id)
-- so every standalone APK presents its own anti-detect phone identity.
ALTER TABLE mobile_apks ADD COLUMN identity_json TEXT NOT NULL DEFAULT '';
ALTER TABLE mobile_apks ADD COLUMN android_id    TEXT NOT NULL DEFAULT '';
