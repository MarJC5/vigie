-- The scan's complements (ADR-0028): the PHP serving the pages as the dashboard noted it, when, or
-- why it was not read; the admin email and the one waiting for confirmation; debug.log's place,
-- size and whether WordPress writes it.

ALTER TABLE scans ADD COLUMN php_web TEXT NOT NULL DEFAULT '';
ALTER TABLE scans ADD COLUMN php_web_seen_at TEXT NOT NULL DEFAULT '';
ALTER TABLE scans ADD COLUMN php_web_unknown TEXT NOT NULL DEFAULT '';
ALTER TABLE scans ADD COLUMN admin_email TEXT NOT NULL DEFAULT '';
ALTER TABLE scans ADD COLUMN admin_email_pending TEXT NOT NULL DEFAULT '';
ALTER TABLE scans ADD COLUMN debug_log_path TEXT NOT NULL DEFAULT '';
ALTER TABLE scans ADD COLUMN debug_log_size INTEGER NOT NULL DEFAULT 0;
ALTER TABLE scans ADD COLUMN debug_log_written INTEGER NOT NULL DEFAULT 0;
