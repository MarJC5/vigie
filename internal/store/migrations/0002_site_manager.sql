-- Site manager: folders (nested), hosts (one SSH account each), sites (one WordPress install each).
-- IDs are version 7 UUIDs as text. A NULL parent means the root. Names are unique among siblings;
-- the application compares them without case (Unicode), these indexes are a safety net.

CREATE TABLE folders (
	id        TEXT PRIMARY KEY,
	parent_id TEXT REFERENCES folders (id) ON DELETE RESTRICT,
	name      TEXT NOT NULL,
	color     INTEGER NOT NULL DEFAULT 0 CHECK (color BETWEEN 0 AND 7),
	notes     TEXT NOT NULL DEFAULT '',
	position  INTEGER NOT NULL DEFAULT 0
) STRICT;
CREATE UNIQUE INDEX folders_sibling_name ON folders (coalesce(parent_id, ''), name COLLATE NOCASE);

CREATE TABLE hosts (
	id                   TEXT PRIMARY KEY,
	folder_id            TEXT REFERENCES folders (id) ON DELETE RESTRICT,
	name                 TEXT NOT NULL,
	alias                TEXT NOT NULL DEFAULT '',
	hostname             TEXT NOT NULL DEFAULT '',
	port                 INTEGER NOT NULL DEFAULT 0,
	user                 TEXT NOT NULL DEFAULT '',
	key_path             TEXT NOT NULL DEFAULT '',
	wp_cli               TEXT NOT NULL DEFAULT '',
	max_concurrent       INTEGER NOT NULL DEFAULT 0,
	host_key_fingerprint TEXT NOT NULL DEFAULT '',
	color                INTEGER NOT NULL DEFAULT 0 CHECK (color BETWEEN 0 AND 7),
	notes                TEXT NOT NULL DEFAULT '',
	position             INTEGER NOT NULL DEFAULT 0
) STRICT;
CREATE UNIQUE INDEX hosts_sibling_name ON hosts (coalesce(folder_id, ''), name COLLATE NOCASE);

CREATE TABLE sites (
	id        TEXT PRIMARY KEY,
	host_id   TEXT NOT NULL REFERENCES hosts (id) ON DELETE RESTRICT,
	name      TEXT NOT NULL,
	path      TEXT NOT NULL DEFAULT '',
	url       TEXT NOT NULL DEFAULT '',
	wp_cli    TEXT NOT NULL DEFAULT '',
	color     INTEGER NOT NULL DEFAULT 0 CHECK (color BETWEEN 0 AND 7),
	notes     TEXT NOT NULL DEFAULT '',
	position  INTEGER NOT NULL DEFAULT 0,
	multisite INTEGER NOT NULL DEFAULT 0 CHECK (multisite IN (0, 1))
) STRICT;
CREATE UNIQUE INDEX sites_sibling_name ON sites (host_id, name COLLATE NOCASE);

CREATE TABLE site_tags (
	site_id  TEXT NOT NULL REFERENCES sites (id) ON DELETE CASCADE,
	position INTEGER NOT NULL,
	tag      TEXT NOT NULL,
	PRIMARY KEY (site_id, tag)
) STRICT;

CREATE TABLE subsites (
	site_id  TEXT NOT NULL REFERENCES sites (id) ON DELETE CASCADE,
	blog_id  INTEGER NOT NULL,
	url      TEXT NOT NULL,
	public   INTEGER NOT NULL DEFAULT 0,
	archived INTEGER NOT NULL DEFAULT 0,
	spam     INTEGER NOT NULL DEFAULT 0,
	deleted  INTEGER NOT NULL DEFAULT 0,
	pinned   INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (site_id, blog_id)
) STRICT;
