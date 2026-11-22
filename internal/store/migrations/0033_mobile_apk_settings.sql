-- Per-APK proxy assignment and notes so standalone apps can route through any saved proxy.
ALTER TABLE mobile_apks ADD COLUMN proxy_id TEXT NOT NULL DEFAULT '';
ALTER TABLE mobile_apks ADD COLUMN notes    TEXT NOT NULL DEFAULT '';
