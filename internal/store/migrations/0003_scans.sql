-- Scans: every scan of a site is kept (history); the latest one, by id (version 7 UUIDs sort by
-- time), feeds the views. Times are RFC 3339 in UTC.

CREATE TABLE scans (
	id                TEXT PRIMARY KEY,
	site_id           TEXT NOT NULL REFERENCES sites (id) ON DELETE CASCADE,
	started_at        TEXT NOT NULL,
	finished_at       TEXT NOT NULL,
	outcome           TEXT NOT NULL CHECK (outcome IN ('completed', 'cannot-load', 'failed', 'cancelled')),
	failure           TEXT NOT NULL DEFAULT '',
	error             TEXT NOT NULL DEFAULT '',
	wordpress         TEXT NOT NULL DEFAULT '',
	php_cli           TEXT NOT NULL DEFAULT '',
	wp_cli            TEXT NOT NULL DEFAULT '',
	home              TEXT NOT NULL DEFAULT '',
	core_checked      INTEGER NOT NULL DEFAULT 0 CHECK (core_checked IN (0, 1)),
	plugins_checked   INTEGER NOT NULL DEFAULT 0 CHECK (plugins_checked IN (0, 1)),
	themes_checked    INTEGER NOT NULL DEFAULT 0 CHECK (themes_checked IN (0, 1)),
	languages_checked INTEGER NOT NULL DEFAULT 0 CHECK (languages_checked IN (0, 1))
) STRICT;
CREATE INDEX scans_site ON scans (site_id, id);

CREATE TABLE scan_items (
	scan_id          TEXT NOT NULL REFERENCES scans (id) ON DELETE CASCADE,
	position         INTEGER NOT NULL,
	kind             TEXT NOT NULL CHECK (kind IN ('core', 'plugin', 'theme', 'language')),
	slug             TEXT NOT NULL DEFAULT '',
	title            TEXT NOT NULL DEFAULT '',
	status           TEXT NOT NULL DEFAULT '',
	version          TEXT NOT NULL DEFAULT '',
	language         TEXT NOT NULL DEFAULT '',
	update_state     TEXT NOT NULL CHECK (update_state IN ('none', 'available', 'unavailable', 'newer-than-source', 'unknown')),
	update_version   TEXT NOT NULL DEFAULT '',
	auto_update      INTEGER NOT NULL DEFAULT 0 CHECK (auto_update IN (0, 1)),
	on_wordpress_org INTEGER NOT NULL DEFAULT 0 CHECK (on_wordpress_org IN (0, 1)),
	PRIMARY KEY (scan_id, position)
) STRICT;
