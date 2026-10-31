-- Runs (plan 09, ADR-0019): each scan of a node, who started it and how it ended, for the Activity
-- page. A scan of a site points to its run; scans made before have none (''). Times are RFC 3339 in
-- UTC, empty while running.

CREATE TABLE runs (
	id          TEXT PRIMARY KEY,
	path        TEXT NOT NULL,
	started_by  TEXT NOT NULL CHECK (started_by IN ('user', 'schedule', 'cli')),
	started_at  TEXT NOT NULL,
	finished_at TEXT NOT NULL DEFAULT '',
	outcome     TEXT NOT NULL CHECK (outcome IN ('running', 'completed', 'cancelled')),
	sites       INTEGER NOT NULL
) STRICT;

ALTER TABLE scans ADD COLUMN run_id TEXT NOT NULL DEFAULT '';
CREATE INDEX scans_run ON scans (run_id);
