-- Reads that stay fast whatever the history kept (an audit on 45,000 scans: seconds before): the
-- latest scan of each site, completed or not, from this index alone; the scans of the journal's
-- days by their start.

CREATE INDEX scans_site_outcome ON scans (site_id, outcome, id);
CREATE INDEX scans_started ON scans (started_at);
