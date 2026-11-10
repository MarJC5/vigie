-- The journal of the assistants' reads of a site's errors: which site, by its path, which client,
-- when. No column holds a text from the log. A removed site keeps its reads through its path, as runs
-- do. Kept as long as runs, never exported.

CREATE TABLE error_reads (
	id        TEXT PRIMARY KEY,
	site_id   TEXT NOT NULL,
	path      TEXT NOT NULL,
	by_client TEXT NOT NULL DEFAULT '',
	read_at   TEXT NOT NULL
) STRICT;
CREATE INDEX error_reads_read ON error_reads (read_at);
