-- The site's backups on the server, as each scan read them (ADR-0033): a JSON list of {at, bytes},
-- oldest first; NULL when the scan did not read them.

ALTER TABLE scans ADD COLUMN backups TEXT;
