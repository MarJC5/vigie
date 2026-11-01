-- What each scan noted of itself when it was made, for the journal (ADR-0030): its status with the
-- ignores of then, and its changes since the complete scan before (JSON list). NULL changes: a scan
-- stored before, computed once at the next start (plan 18).

ALTER TABLE scans ADD COLUMN recorded_status TEXT NOT NULL DEFAULT '';
ALTER TABLE scans ADD COLUMN recorded_changes TEXT;
