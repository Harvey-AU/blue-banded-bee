-- Add traffic_score column to page_analytics
-- Stores pre-calculated score based on page view percentile within the domain
-- Score values: 0.95 (top 5%), 0.90 (top 10%), 0.75 (top 25%), 0.50 (top 50%), 0 (bottom 50%)

ALTER TABLE page_analytics
    ADD COLUMN traffic_score FLOAT DEFAULT 0;

-- Index for efficient lookups during priority updates
CREATE INDEX idx_page_analytics_traffic_score ON page_analytics(domain_id, path, traffic_score);

COMMENT ON COLUMN page_analytics.traffic_score IS 'Pre-calculated priority score based on page view percentile: 0.95 (top 5%), 0.90 (top 10%), 0.75 (top 25%), 0.50 (top 50%), 0 (bottom 50%)';
