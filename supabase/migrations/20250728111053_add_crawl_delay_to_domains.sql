-- Add crawl_delay_seconds column to domains table
ALTER TABLE domains 
ADD COLUMN IF NOT EXISTS crawl_delay_seconds INTEGER DEFAULT NULL;

-- Add comment for documentation
COMMENT ON COLUMN domains.crawl_delay_seconds IS 'Crawl delay in seconds from robots.txt for this domain';