-- Comprehensive Initial Schema for Blue Banded Bee
-- This migration creates the complete schema based on the setupSchema() function in internal/db/db.go
-- and incorporates all enhancements from subsequent migrations

-- Enable necessary extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- =============================================================================
-- TABLE CREATION
-- =============================================================================

-- Create organisations table first (referenced by users and jobs)
CREATE TABLE IF NOT EXISTS organisations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create users table (extends Supabase auth.users)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL,
    full_name TEXT,
    organisation_id UUID REFERENCES organisations(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(email)
);

-- Create domains lookup table
CREATE TABLE IF NOT EXISTS domains (
    id SERIAL PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    crawl_delay_seconds INTEGER DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create pages lookup table
CREATE TABLE IF NOT EXISTS pages (
    id SERIAL PRIMARY KEY,
    domain_id INTEGER NOT NULL REFERENCES domains(id),
    path TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(domain_id, path)
);

-- Create jobs table with generated columns for duration calculations
CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    domain_id INTEGER NOT NULL REFERENCES domains(id),
    user_id UUID REFERENCES users(id),
    organisation_id UUID REFERENCES organisations(id),
    status TEXT NOT NULL,
    progress REAL NOT NULL,
    sitemap_tasks INTEGER NOT NULL DEFAULT 0,
    found_tasks INTEGER NOT NULL DEFAULT 0,
    total_tasks INTEGER NOT NULL DEFAULT 0,
    completed_tasks INTEGER NOT NULL DEFAULT 0,
    failed_tasks INTEGER NOT NULL DEFAULT 0,
    skipped_tasks INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    concurrency INTEGER NOT NULL,
    find_links BOOLEAN NOT NULL,
    max_pages INTEGER NOT NULL,
    include_paths TEXT,
    exclude_paths TEXT,
    required_workers INTEGER DEFAULT 0,
    error_message TEXT,
    source_type TEXT,
    source_detail TEXT,
    source_info TEXT,
    -- Generated columns for duration calculations
    duration_seconds INTEGER GENERATED ALWAYS AS (
        CASE 
            WHEN started_at IS NOT NULL AND completed_at IS NOT NULL 
            THEN EXTRACT(EPOCH FROM (completed_at - started_at))::INTEGER
            ELSE NULL
        END
    ) STORED,
    avg_time_per_task_seconds NUMERIC GENERATED ALWAYS AS (
        CASE 
            WHEN started_at IS NOT NULL AND completed_at IS NOT NULL AND completed_tasks > 0 
            THEN EXTRACT(EPOCH FROM (completed_at - started_at))::NUMERIC / completed_tasks::NUMERIC
            ELSE NULL
        END
    ) STORED
);

-- Create tasks table
CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL,
    page_id INTEGER NOT NULL REFERENCES pages(id),
    path TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    retry_count INTEGER NOT NULL,
    error TEXT,
    source_type TEXT NOT NULL,
    source_url TEXT,
    status_code INTEGER,
    response_time BIGINT,
    cache_status TEXT,
    content_type TEXT,
    content_length BIGINT,
    headers JSONB,
    redirect_url TEXT,
    dns_lookup_time INTEGER,
    tcp_connection_time INTEGER,
    tls_handshake_time INTEGER,
    ttfb INTEGER,
    content_transfer_time INTEGER,
    second_response_time BIGINT,
    second_cache_status TEXT,
    second_content_length BIGINT,
    second_headers JSONB,
    second_dns_lookup_time INTEGER,
    second_tcp_connection_time INTEGER,
    second_tls_handshake_time INTEGER,
    second_ttfb INTEGER,
    second_content_transfer_time INTEGER,
    cache_check_attempts JSONB,
    priority_score NUMERIC(4,3) DEFAULT 0.000,
    FOREIGN KEY (job_id) REFERENCES jobs(id)
);

-- =============================================================================
-- INDEXES
-- =============================================================================

-- Core task indexes
CREATE INDEX IF NOT EXISTS idx_tasks_job_id ON tasks(job_id);

-- Optimised index for worker task claiming (most important for performance)
CREATE INDEX IF NOT EXISTS idx_tasks_pending_claim_order ON tasks (created_at) WHERE status = 'pending';

-- Index for dashboard/API queries on job status and priority
CREATE INDEX IF NOT EXISTS idx_tasks_job_status_priority ON tasks(job_id, status, priority_score DESC);

-- Unique constraint to prevent duplicate tasks for same job/page combination
CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_job_page_unique ON tasks(job_id, page_id);

-- Job performance indexes
CREATE INDEX IF NOT EXISTS idx_jobs_duration ON jobs(duration_seconds) 
WHERE duration_seconds IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_jobs_avg_time ON jobs(avg_time_per_task_seconds) 
WHERE avg_time_per_task_seconds IS NOT NULL;

-- Job lookup index for efficient updates
CREATE INDEX IF NOT EXISTS idx_jobs_id ON jobs(id);

-- =============================================================================
-- ROW LEVEL SECURITY (RLS)
-- =============================================================================

-- Enable RLS on all tables
ALTER TABLE organisations ENABLE ROW LEVEL SECURITY;
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE domains ENABLE ROW LEVEL SECURITY;
ALTER TABLE pages ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE tasks ENABLE ROW LEVEL SECURITY;

-- Create RLS policies
-- Users can only access their own data
DROP POLICY IF EXISTS "Users can access own data" ON users;
CREATE POLICY "Users can access own data" ON users
FOR ALL USING (auth.uid() = id);

-- Users can access their organisation
DROP POLICY IF EXISTS "Users can access own organisation" ON organisations;
CREATE POLICY "Users can access own organisation" ON organisations
FOR ALL USING (
    id IN (
        SELECT organisation_id FROM users WHERE id = auth.uid()
    )
);

-- Organisation members can access shared jobs
DROP POLICY IF EXISTS "Organisation members can access jobs" ON jobs;
CREATE POLICY "Organisation members can access jobs" ON jobs
FOR ALL USING (
    organisation_id IN (
        SELECT organisation_id FROM users WHERE id = auth.uid()
    )
);

-- Organisation members can access tasks for their jobs
DROP POLICY IF EXISTS "Organisation members can access tasks" ON tasks;
CREATE POLICY "Organisation members can access tasks" ON tasks
FOR ALL USING (
    job_id IN (
        SELECT id FROM jobs WHERE organisation_id IN (
            SELECT organisation_id FROM users WHERE id = auth.uid()
        )
    )
);

-- =============================================================================
-- TRIGGER FUNCTIONS
-- =============================================================================

-- Function to automatically set started_at when first task completes
CREATE OR REPLACE FUNCTION set_job_started_at()
RETURNS TRIGGER AS $$
BEGIN
  -- Only set started_at if it's currently NULL and completed_tasks > 0
  -- Handle both INSERT and UPDATE operations
  IF NEW.completed_tasks > 0 AND (TG_OP = 'INSERT' OR OLD.started_at IS NULL) AND NEW.started_at IS NULL THEN
    NEW.started_at = CURRENT_TIMESTAMP AT TIME ZONE 'UTC';
  END IF;
  
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Function to automatically set completed_at when job reaches 100%
CREATE OR REPLACE FUNCTION set_job_completed_at()
RETURNS TRIGGER AS $$
BEGIN
  -- Set completed_at when progress reaches 100% and it's not already set
  -- Handle both INSERT and UPDATE operations
  IF NEW.progress >= 100.0 AND (TG_OP = 'INSERT' OR OLD.completed_at IS NULL) AND NEW.completed_at IS NULL THEN
    NEW.completed_at = CURRENT_TIMESTAMP AT TIME ZONE 'UTC';
  END IF;
  
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- High-performance O(1) incremental counter function for job progress
CREATE OR REPLACE FUNCTION update_job_counters()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        -- New task created - increment total_tasks and source-specific counters
        UPDATE jobs 
        SET total_tasks = total_tasks + 1,
            sitemap_tasks = CASE 
                WHEN NEW.source_type = 'sitemap' THEN sitemap_tasks + 1 
                ELSE sitemap_tasks 
            END,
            found_tasks = CASE 
                WHEN NEW.source_type != 'sitemap' OR NEW.source_type IS NULL THEN found_tasks + 1 
                ELSE found_tasks 
            END
        WHERE id = NEW.job_id;
        
    ELSIF TG_OP = 'UPDATE' AND OLD.status != NEW.status THEN
        -- Status changed - update status counters and progress
        UPDATE jobs 
        SET completed_tasks = completed_tasks + 
                CASE WHEN NEW.status = 'completed' AND OLD.status != 'completed' THEN 1 
                     WHEN OLD.status = 'completed' AND NEW.status != 'completed' THEN -1 
                     ELSE 0 END,
            failed_tasks = failed_tasks + 
                CASE WHEN NEW.status = 'failed' AND OLD.status != 'failed' THEN 1 
                     WHEN OLD.status = 'failed' AND NEW.status != 'failed' THEN -1 
                     ELSE 0 END,
            skipped_tasks = skipped_tasks + 
                CASE WHEN NEW.status = 'skipped' AND OLD.status != 'skipped' THEN 1 
                     WHEN OLD.status = 'skipped' AND NEW.status != 'skipped' THEN -1 
                     ELSE 0 END,
            progress = CASE 
                WHEN total_tasks > 0 AND (total_tasks - skipped_tasks) > 0 THEN
                    ((completed_tasks + failed_tasks)::REAL / (total_tasks - skipped_tasks)::REAL) * 100.0
                ELSE 0.0 
            END,
            -- Update timestamps when job starts/completes
            started_at = CASE 
                WHEN started_at IS NULL AND NEW.status = 'running' THEN NOW()
                ELSE started_at
            END,
            completed_at = CASE 
                WHEN NEW.status IN ('completed', 'failed') AND 
                     completed_tasks + failed_tasks + skipped_tasks >= total_tasks THEN NOW()
                ELSE completed_at
            END
        WHERE id = NEW.job_id;
        
    ELSIF TG_OP = 'DELETE' THEN
        -- Task deleted - decrement counters
        UPDATE jobs 
        SET total_tasks = GREATEST(0, total_tasks - 1),
            completed_tasks = CASE 
                WHEN OLD.status = 'completed' THEN GREATEST(0, completed_tasks - 1) 
                ELSE completed_tasks 
            END,
            failed_tasks = CASE 
                WHEN OLD.status = 'failed' THEN GREATEST(0, failed_tasks - 1) 
                ELSE failed_tasks 
            END,
            skipped_tasks = CASE 
                WHEN OLD.status = 'skipped' THEN GREATEST(0, skipped_tasks - 1) 
                ELSE skipped_tasks 
            END,
            sitemap_tasks = CASE 
                WHEN OLD.source_type = 'sitemap' THEN GREATEST(0, sitemap_tasks - 1) 
                ELSE sitemap_tasks 
            END,
            found_tasks = CASE 
                WHEN OLD.source_type != 'sitemap' OR OLD.source_type IS NULL THEN GREATEST(0, found_tasks - 1) 
                ELSE found_tasks 
            END
        WHERE id = OLD.job_id;
    END IF;
    
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Function for manual job stats recalculation (used by Go code)
CREATE OR REPLACE FUNCTION recalculate_job_stats(p_job_id TEXT) 
RETURNS void AS $$
BEGIN
    UPDATE jobs 
    SET total_tasks = COALESCE((SELECT COUNT(*) FROM tasks WHERE job_id = p_job_id), 0),
        completed_tasks = COALESCE((SELECT COUNT(*) FROM tasks WHERE job_id = p_job_id AND status = 'completed'), 0),
        failed_tasks = COALESCE((SELECT COUNT(*) FROM tasks WHERE job_id = p_job_id AND status = 'failed'), 0),
        skipped_tasks = COALESCE((SELECT COUNT(*) FROM tasks WHERE job_id = p_job_id AND status = 'skipped'), 0),
        progress = CASE 
            WHEN total_tasks > 0 AND (total_tasks - skipped_tasks) > 0 THEN
                ((completed_tasks + failed_tasks)::REAL / (total_tasks - skipped_tasks)::REAL) * 100.0
            ELSE 0.0 
        END
    WHERE id = p_job_id;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- TRIGGERS
-- =============================================================================

-- Trigger for setting started_at timestamps
DROP TRIGGER IF EXISTS trigger_set_job_started ON jobs;
CREATE TRIGGER trigger_set_job_started
  BEFORE INSERT OR UPDATE ON jobs
  FOR EACH ROW
  EXECUTE FUNCTION set_job_started_at();

-- Trigger for setting completed_at timestamps
DROP TRIGGER IF EXISTS trigger_set_job_completed ON jobs;
CREATE TRIGGER trigger_set_job_completed
  BEFORE INSERT OR UPDATE ON jobs
  FOR EACH ROW
  EXECUTE FUNCTION set_job_completed_at();

-- High-performance trigger for job progress updates
DROP TRIGGER IF EXISTS trigger_update_job_counters ON tasks;
CREATE TRIGGER trigger_update_job_counters
    AFTER INSERT OR UPDATE OF status OR DELETE ON tasks
    FOR EACH ROW
    EXECUTE FUNCTION update_job_counters();

-- =============================================================================
-- COMMENTS FOR DOCUMENTATION
-- =============================================================================

COMMENT ON COLUMN domains.crawl_delay_seconds IS 'Crawl delay in seconds from robots.txt for this domain';
COMMENT ON COLUMN jobs.duration_seconds IS 'Total job duration in seconds (calculated from started_at to completed_at)';
COMMENT ON COLUMN jobs.avg_time_per_task_seconds IS 'Average time per completed task in seconds';
COMMENT ON FUNCTION update_job_counters() IS 'High-performance O(1) incremental job progress tracking';
COMMENT ON FUNCTION recalculate_job_stats(TEXT) IS 'Recalculates all job statistics from actual task records for data consistency';