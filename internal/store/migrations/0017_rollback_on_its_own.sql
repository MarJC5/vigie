-- Plugins and themes that break a site roll back on their own (ADR-0042). The rollback is an
-- operation of its own: of_operation names the update it came from, automatic tells that Vigie
-- started it. A version rolled back is set aside on its site, until a newer one is offered or the
-- user updates it alone: one row per site and item.

ALTER TABLE operations ADD COLUMN of_operation TEXT NOT NULL DEFAULT '';
ALTER TABLE operations ADD COLUMN automatic INTEGER NOT NULL DEFAULT 0 CHECK (automatic IN (0, 1));

CREATE TABLE set_asides (
	site_id      TEXT NOT NULL REFERENCES sites (id) ON DELETE CASCADE,
	item_kind    TEXT NOT NULL CHECK (item_kind IN ('plugin', 'theme')),
	slug         TEXT NOT NULL CHECK (slug <> ''),
	version      TEXT NOT NULL CHECK (version <> ''),
	operation_id TEXT NOT NULL,
	reason       TEXT NOT NULL DEFAULT '',
	created_at   TEXT NOT NULL,
	PRIMARY KEY (site_id, item_kind, slug)
) STRICT;
