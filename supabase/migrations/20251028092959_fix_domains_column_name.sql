-- Fix domains table column name mismatch
-- The schema expects 'crawl_delay_seconds' but database has 'crawl_delay'
-- This likely occurred during a manual schema reset or modification

-- Rename the column to match what the code expects
ALTER TABLE domains
RENAME COLUMN crawl_delay TO crawl_delay_seconds;

-- Update the column comment to match
COMMENT ON COLUMN domains.crawl_delay_seconds IS 'Crawl delay in seconds from robots.txt for this domain';
