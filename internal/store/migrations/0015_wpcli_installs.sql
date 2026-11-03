-- The installs of WP-CLI on an account by Vigie (ADR-0039), told in the journal. An install can
-- come before its host is stored (the add-host wizard): it is linked to the host by its SSH
-- account, not by an identifier.

CREATE TABLE wpcli_installs (
	id          TEXT PRIMARY KEY,
	account     TEXT NOT NULL,
	host_name   TEXT NOT NULL,
	started_at  TEXT NOT NULL,
	finished_at TEXT NOT NULL,
	completed   INTEGER NOT NULL CHECK (completed IN (0, 1)),
	version     TEXT NOT NULL,
	command     TEXT NOT NULL DEFAULT '',
	failed_step TEXT NOT NULL DEFAULT '' CHECK (failed_step IN ('', 'download', 'check', 'send', 'test')),
	message     TEXT NOT NULL DEFAULT ''
) STRICT;
CREATE INDEX wpcli_installs_started ON wpcli_installs (started_at);
