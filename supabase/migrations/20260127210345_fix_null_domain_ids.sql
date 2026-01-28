-- Fix NULL domain_ids that cause pq.Array() scanning errors
-- This resolves: "pq: scanning to int is not implemented; only sql.Scanner"
UPDATE google_analytics_connections
SET domain_ids = '{}'::integer[]
WHERE domain_ids IS NULL;
