-- Standalone APK app catalog for the Pro APK tab.
CREATE TABLE IF NOT EXISTS mobile_apks (
	package         TEXT PRIMARY KEY,
	label           TEXT NOT NULL,
	version_name    TEXT NOT NULL DEFAULT '1.0',
	version_code    INTEGER NOT NULL DEFAULT 0,
	launch_activity TEXT NOT NULL DEFAULT '',
	orientation     TEXT NOT NULL DEFAULT 'auto',
	abis_json       TEXT NOT NULL DEFAULT '[]',
	file_path       TEXT NOT NULL DEFAULT '',
	icon_data       TEXT NOT NULL DEFAULT '',
	device_id       TEXT NOT NULL DEFAULT '',
	shortcut_path   TEXT NOT NULL DEFAULT '',
	installed_at    TEXT NOT NULL,
	last_open_at    TEXT NOT NULL DEFAULT ''
);
