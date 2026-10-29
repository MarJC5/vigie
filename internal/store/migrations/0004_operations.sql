-- Operations that write on a server (updates, rollbacks), recorded step by step: the log of what
-- Vigie did. Times are RFC 3339 in UTC, empty while running. pid is the process running the
-- operation: one that ended leaves a "running" row, marked interrupted by the next operation.

CREATE TABLE operations (
	id          TEXT PRIMARY KEY,
	site_id     TEXT NOT NULL REFERENCES sites (id) ON DELETE CASCADE,
	kind        TEXT NOT NULL CHECK (kind IN ('update', 'rollback')),
	started_at  TEXT NOT NULL,
	finished_at TEXT NOT NULL DEFAULT '',
	outcome     TEXT NOT NULL CHECK (outcome IN ('running', 'completed', 'failed', 'cancelled', 'interrupted')),
	summary     TEXT NOT NULL DEFAULT '',
	backup_dir  TEXT NOT NULL DEFAULT '',
	pid         INTEGER NOT NULL DEFAULT 0
) STRICT;
CREATE INDEX operations_site ON operations (site_id, id);

CREATE TABLE operation_steps (
	operation_id   TEXT NOT NULL REFERENCES operations (id) ON DELETE CASCADE,
	position       INTEGER NOT NULL,
	step           TEXT NOT NULL CHECK (step IN ('backup-database', 'backup-files', 'maintenance-on', 'update',
		'update-database', 'check', 'maintenance-off', 'restore-files', 'restore-database')),
	item_kind      TEXT NOT NULL DEFAULT '' CHECK (item_kind IN ('', 'core', 'plugin', 'theme', 'language')),
	slug           TEXT NOT NULL DEFAULT '',
	version_before TEXT NOT NULL DEFAULT '',
	version_after  TEXT NOT NULL DEFAULT '',
	started_at     TEXT NOT NULL,
	finished_at    TEXT NOT NULL DEFAULT '',
	outcome        TEXT NOT NULL CHECK (outcome IN ('running', 'done', 'failed', 'skipped')),
	message        TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (operation_id, position)
) STRICT;
