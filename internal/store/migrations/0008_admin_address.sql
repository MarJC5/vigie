-- The admin address of a site (ADR-0023, amended on 2026-11-01): the WordPress address a scan
-- reads (siteurl, where wp-admin lives, a subfolder of the site at times), and the address the
-- user gives for a login a plugin renamed.

ALTER TABLE scans ADD COLUMN site_url TEXT NOT NULL DEFAULT '';
ALTER TABLE sites ADD COLUMN admin_url TEXT NOT NULL DEFAULT '';
