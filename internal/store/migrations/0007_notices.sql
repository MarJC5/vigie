-- Notifications sent (plan 09, ADR-0019): what Vigie told, after which run, for the Activity page.
-- names is a JSON array of strings; times are RFC 3339 in UTC.

CREATE TABLE notices (
	id      TEXT PRIMARY KEY,
	run_id  TEXT NOT NULL DEFAULT '',
	kind    TEXT NOT NULL,
	names   TEXT NOT NULL,
	sent_at TEXT NOT NULL
) STRICT;
CREATE INDEX notices_sent ON notices (sent_at);
