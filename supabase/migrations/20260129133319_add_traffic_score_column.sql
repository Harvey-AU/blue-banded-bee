-- Add traffic_score column to page_analytics
-- Stores pre-calculated score based on page view percentile within the domain
-- Score values: log-scaled 0.10-0.99 (0 for 0-1 views)

ALTER TABLE page_analytics
    ADD COLUMN traffic_score FLOAT DEFAULT 0;

-- Index for efficient lookups during priority updates
CREATE INDEX IF NOT EXISTS idx_page_analytics_traffic_score_v2 ON page_analytics(organisation_id, domain_id, path, traffic_score);

COMMENT ON COLUMN page_analytics.traffic_score IS 'Pre-calculated priority score based on log-scaled 28-day view curve (0.10-0.99); 0 for 0-1 views';
