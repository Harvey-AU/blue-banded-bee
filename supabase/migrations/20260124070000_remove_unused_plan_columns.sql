-- Remove unused plan columns that were never enforced
-- These can be re-added if/when the features are implemented

ALTER TABLE plans DROP COLUMN IF EXISTS max_concurrent_jobs;
ALTER TABLE plans DROP COLUMN IF EXISTS max_pages_per_job;
