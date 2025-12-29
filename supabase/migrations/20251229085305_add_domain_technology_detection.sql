-- Add technology detection columns to domains table
-- Stores detected technologies (CMS, CDN, frameworks, etc.) and raw response data for debugging

ALTER TABLE domains
    ADD COLUMN IF NOT EXISTS technologies JSONB DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS tech_headers JSONB DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS tech_html_sample TEXT DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS tech_detected_at TIMESTAMPTZ DEFAULT NULL;

-- Add index for querying domains by technology
CREATE INDEX IF NOT EXISTS idx_domains_technologies ON domains USING GIN (technologies);

-- Add comment explaining the columns
COMMENT ON COLUMN domains.technologies IS 'Detected technologies with categories from wappalyzergo (e.g., {"Cloudflare": ["CDN"], "WordPress": ["CMS"]})';
COMMENT ON COLUMN domains.tech_headers IS 'Raw HTTP headers from detection request for debugging';
COMMENT ON COLUMN domains.tech_html_sample IS 'Truncated HTML sample (first 50KB) from detection request for debugging';
COMMENT ON COLUMN domains.tech_detected_at IS 'Timestamp of last technology detection run';
