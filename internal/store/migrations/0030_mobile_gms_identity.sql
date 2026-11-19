-- Add Google Mobile Services (GMS) toggle and per-device identity profile JSON
-- to mobile_devices. Existing rows default to with_gms=0 and identity_json=''
-- so devices created before this upgrade keep working unchanged.
ALTER TABLE mobile_devices ADD COLUMN with_gms INTEGER NOT NULL DEFAULT 0;
ALTER TABLE mobile_devices ADD COLUMN identity_json TEXT NOT NULL DEFAULT '';
