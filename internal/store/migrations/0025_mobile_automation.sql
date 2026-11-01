-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Automation settings per device. Existing devices keep the balanced preset
-- and the screen they already run at; new devices may choose a lighter one.
ALTER TABLE mobile_devices ADD COLUMN preset TEXT NOT NULL DEFAULT 'balanced';
ALTER TABLE mobile_devices ADD COLUMN screen_w INTEGER NOT NULL DEFAULT 720;
ALTER TABLE mobile_devices ADD COLUMN screen_h INTEGER NOT NULL DEFAULT 1600;
ALTER TABLE mobile_devices ADD COLUMN density INTEGER NOT NULL DEFAULT 280;
