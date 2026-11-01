-- What each subsite of a network had active, and who administered it, at each scan (ADR-0029).
-- network_plugins is the JSON list of the plugins active network-wide; NULL when the scan read no
-- subsites (a single site, or a read that failed).

ALTER TABLE scans ADD COLUMN network_plugins TEXT;

CREATE TABLE scan_subsites (
	scan_id       TEXT NOT NULL REFERENCES scans (id) ON DELETE CASCADE,
	blog_id       INTEGER NOT NULL,
	theme         TEXT NOT NULL DEFAULT '',
	theme_name    TEXT NOT NULL DEFAULT '',
	theme_version TEXT NOT NULL DEFAULT '',
	admin_email   TEXT NOT NULL DEFAULT '',
	plugins       TEXT NOT NULL DEFAULT '[]', -- JSON list of slugs
	admins        TEXT NOT NULL DEFAULT '[]', -- JSON list of logins
	PRIMARY KEY (scan_id, blog_id)
) STRICT;
