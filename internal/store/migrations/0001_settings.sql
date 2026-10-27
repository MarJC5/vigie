-- Application settings (language, update checks, data choices), one value per key.
CREATE TABLE settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
) STRICT;
