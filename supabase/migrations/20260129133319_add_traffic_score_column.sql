-- Add traffic_score column to page_analytics
-- Stores pre-calculated score based on page view percentile within the domain
-- Score values: 0.95 (top 5%), 0.90 (top 10%), 0.75 (top 25%), 0.50 (top 50%), 0 (bottom 50%)

ALTER TABLE page_analytics
    ADD COLUMN traffic_score FLOAT DEFAULT 0;

-- Index for efficient lookups during priority updates
DROP INDEX IF EXISTS idx_page_analytics_traffic_score;
CREATE INDEX idx_page_analytics_traffic_score ON page_analytics(organisation_id, domain_id, path, traffic_score);

COMMENT ON COLUMN page_analytics.traffic_score IS 'Pre-calculated priority score based on log-scaled 28-day view curve (0.10-0.99); 0 for 0-1 views';
