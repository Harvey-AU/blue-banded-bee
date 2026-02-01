-- Fix NULL domain_ids that cause pq.Array() scanning errors
-- This resolves: "pq: scanning to int is not implemented; only sql.Scanner"
UPDATE google_analytics_connections
SET domain_ids = '{}'::integer[]
WHERE domain_ids IS NULL;

-- Enforce NOT NULL constraint to prevent future NULLs
ALTER TABLE google_analytics_connections
ALTER COLUMN domain_ids SET NOT NULL;
