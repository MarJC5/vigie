-- Security scan (plan 05, ADR-0014 and ADR-0015): the findings of each scan, the ignores of each
-- site, and a cache of the answers of outside lookups. Times are RFC 3339 in UTC.

-- When the files were last compared with wordpress.org's; empty when never.
ALTER TABLE scans ADD COLUMN files_checked_at TEXT NOT NULL DEFAULT '';

-- A finding holds data, not text: facts is a JSON object of strings.
CREATE TABLE scan_findings (
	scan_id    TEXT NOT NULL REFERENCES scans (id) ON DELETE CASCADE,
	position   INTEGER NOT NULL,
	kind       TEXT NOT NULL,
	subject    TEXT NOT NULL DEFAULT '',
	identifier TEXT NOT NULL DEFAULT '',
	version    TEXT NOT NULL DEFAULT '',
	level      TEXT NOT NULL CHECK (level IN ('urgent', 'attention', 'info', 'not-checked')),
	facts      TEXT NOT NULL DEFAULT '{}',
	PRIMARY KEY (scan_id, position)
) STRICT;

-- An ignore lasts until the version it was made for changes, or, with an empty until_version,
-- permanently, and then a reason is required.
CREATE TABLE ignores (
	site_id       TEXT NOT NULL REFERENCES sites (id) ON DELETE CASCADE,
	finding_key   TEXT NOT NULL,
	until_version TEXT NOT NULL DEFAULT '',
	reason        TEXT NOT NULL DEFAULT '',
	created_at    TEXT NOT NULL,
	CHECK (until_version <> '' OR trim(reason) <> ''),
	PRIMARY KEY (site_id, finding_key)
) STRICT;

-- The last answer of an outside lookup: source is the service (wpvulnerability, wordpress.org,
-- rdap...), key what was asked (plugin/<slug>, a domain...), status the HTTP status.
CREATE TABLE lookup_cache (
	source     TEXT NOT NULL,
	key        TEXT NOT NULL,
	status     INTEGER NOT NULL,
	body       BLOB NOT NULL,
	fetched_at TEXT NOT NULL,
	PRIMARY KEY (source, key)
) STRICT;
