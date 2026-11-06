-- Who started a run (ADR-0043): an assistant, through vigie mcp, joins the window, the automatic
-- scans and the command, with its client's name when it gives one. Scans already had their origin
-- (runs.started_by, whose CHECK SQLite cannot widen: the table is rebuilt); updates, rollbacks and
-- copies gain it, an older row reading as started from the window.

CREATE TABLE runs_0018 (
	id          TEXT PRIMARY KEY,
	path        TEXT NOT NULL,
	started_by  TEXT NOT NULL CHECK (started_by IN ('user', 'schedule', 'cli', 'assistant')),
	by_client   TEXT NOT NULL DEFAULT '',
	started_at  TEXT NOT NULL,
	finished_at TEXT NOT NULL DEFAULT '',
	outcome     TEXT NOT NULL CHECK (outcome IN ('running', 'completed', 'cancelled')),
	sites       INTEGER NOT NULL
) STRICT;
INSERT INTO runs_0018 (id, path, started_by, started_at, finished_at, outcome, sites)
	SELECT id, path, started_by, started_at, finished_at, outcome, sites FROM runs;
DROP TABLE runs;
ALTER TABLE runs_0018 RENAME TO runs;

ALTER TABLE operations ADD COLUMN started_by TEXT NOT NULL DEFAULT 'user'
	CHECK (started_by IN ('user', 'schedule', 'cli', 'assistant'));
ALTER TABLE operations ADD COLUMN by_client TEXT NOT NULL DEFAULT '';

ALTER TABLE copies ADD COLUMN started_by TEXT NOT NULL DEFAULT 'user'
	CHECK (started_by IN ('user', 'schedule', 'cli', 'assistant'));
ALTER TABLE copies ADD COLUMN by_client TEXT NOT NULL DEFAULT '';
