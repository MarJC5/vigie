-- The copies of sites to the user's computer (ADR-0041), and their resumes, told in the journal.
-- The copies themselves are files in the folder of the copies; Vigie lists them from there.

CREATE TABLE copies (
	id          TEXT PRIMARY KEY,
	site_id     TEXT NOT NULL,
	site_name   TEXT NOT NULL,
	host_name   TEXT NOT NULL,
	folder      TEXT NOT NULL,
	content     TEXT NOT NULL CHECK (content IN ('database', 'content', 'site')),
	resumed     INTEGER NOT NULL CHECK (resumed IN (0, 1)),
	started_at  TEXT NOT NULL,
	finished_at TEXT NOT NULL,
	outcome     TEXT NOT NULL CHECK (outcome IN ('completed', 'failed', 'cancelled')),
	archives    INTEGER NOT NULL,
	bytes       INTEGER NOT NULL,
	message     TEXT NOT NULL DEFAULT ''
) STRICT;
CREATE INDEX copies_started ON copies (started_at);
