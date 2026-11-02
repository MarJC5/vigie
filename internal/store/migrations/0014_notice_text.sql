-- What the notification of a run's end said (ADR-0035), shown as it was sent in the journal; empty
-- for the alerts, told from their kind and names.

ALTER TABLE notices ADD COLUMN text TEXT NOT NULL DEFAULT '';
