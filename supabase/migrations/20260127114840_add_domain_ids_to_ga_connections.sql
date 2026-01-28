-- Add domain_ids array column to google_analytics_connections
-- This allows mapping GA4 properties to specific domains

ALTER TABLE google_analytics_connections
ADD COLUMN IF NOT EXISTS domain_ids INTEGER[] DEFAULT '{}'::integer[];

-- Add comment for documentation
COMMENT ON COLUMN google_analytics_connections.domain_ids IS
'Array of domain IDs (from domains table) associated with this GA4 property connection';

-- Add GIN index for efficient array membership queries
CREATE INDEX IF NOT EXISTS idx_ga_connections_domain_ids
ON google_analytics_connections USING GIN(domain_ids);
