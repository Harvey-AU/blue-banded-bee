-- Migration: Create schedulers table for recurring job scheduling
-- Adds schedulers table and scheduler_id column to jobs table

-- Create schedulers table
CREATE TABLE IF NOT EXISTS schedulers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id INTEGER NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
    schedule_interval_hours INTEGER NOT NULL CHECK (schedule_interval_hours IN (6, 12, 24, 48)),
    next_run_at TIMESTAMPTZ NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    
    -- Job configuration template
    concurrency INTEGER NOT NULL DEFAULT 20,
    find_links BOOLEAN NOT NULL DEFAULT TRUE,
    max_pages INTEGER NOT NULL DEFAULT 0,
    include_paths TEXT,
    exclude_paths TEXT,
    required_workers INTEGER NOT NULL DEFAULT 1,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT unique_domain_org UNIQUE(domain_id, organisation_id)
);

-- Index for efficient querying of schedulers ready to run
CREATE INDEX IF NOT EXISTS idx_schedulers_next_run 
ON schedulers(next_run_at) 
WHERE is_enabled = TRUE;

-- Index for organisation queries
CREATE INDEX IF NOT EXISTS idx_schedulers_organisation 
ON schedulers(organisation_id);

-- Add scheduler_id to jobs table to link executions back to scheduler
ALTER TABLE jobs 
ADD COLUMN IF NOT EXISTS scheduler_id UUID REFERENCES schedulers(id);

-- Index for querying job execution history per scheduler
CREATE INDEX IF NOT EXISTS idx_jobs_scheduler_id 
ON jobs(scheduler_id) 
WHERE scheduler_id IS NOT NULL;

-- Comments for documentation
COMMENT ON TABLE schedulers IS 'Stores recurring job schedule configurations';
COMMENT ON COLUMN schedulers.schedule_interval_hours IS 'Recurring job interval in hours (6, 12, 24, or 48)';
COMMENT ON COLUMN schedulers.next_run_at IS 'Next scheduled execution time';
COMMENT ON COLUMN jobs.scheduler_id IS 'Links job execution to its scheduler (NULL for manual jobs)';


