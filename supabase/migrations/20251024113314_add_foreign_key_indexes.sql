-- Add foreign key indexes to improve query performance and reduce lock contention
-- Note: CONCURRENTLY removed as Supabase migrations run in transactions
-- Context: Load test revealed missing indexes causing table scans and 2-6 second lock waits

-- High priority: jobs table (heavily queried for dashboard, task claiming)
CREATE INDEX IF NOT EXISTS idx_jobs_domain_id
ON jobs(domain_id);

CREATE INDEX IF NOT EXISTS idx_jobs_user_id
ON jobs(user_id);

CREATE INDEX IF NOT EXISTS idx_jobs_organisation_id
ON jobs(organisation_id);

-- Critical: composite index for task claiming (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_tasks_job_id_status
ON tasks(job_id, status);

-- Medium priority: tasks table page lookups
CREATE INDEX IF NOT EXISTS idx_tasks_page_id
ON tasks(page_id);

-- Low priority: user and share link lookups
CREATE INDEX IF NOT EXISTS idx_users_organisation_id
ON users(organisation_id);

CREATE INDEX IF NOT EXISTS idx_job_share_links_created_by
ON job_share_links(created_by);

-- Add index on job status for dashboard queries
CREATE INDEX IF NOT EXISTS idx_jobs_status_created_at
ON jobs(status, created_at DESC);

-- Add index on task status for worker pool queries
CREATE INDEX IF NOT EXISTS idx_tasks_status_priority
ON tasks(status, priority_score DESC, created_at ASC)
WHERE status = 'pending';

COMMENT ON INDEX idx_jobs_domain_id IS 'Improves domain-based job lookups and joins';
COMMENT ON INDEX idx_jobs_user_id IS 'Improves user job filtering';
COMMENT ON INDEX idx_jobs_organisation_id IS 'Improves organisation job filtering and RLS checks';
COMMENT ON INDEX idx_tasks_job_id_status IS 'Critical for task claiming and job progress queries';
COMMENT ON INDEX idx_tasks_page_id IS 'Improves page-based task lookups';
COMMENT ON INDEX idx_users_organisation_id IS 'Improves user-organisation joins';
COMMENT ON INDEX idx_job_share_links_created_by IS 'Improves share link creator lookups';
COMMENT ON INDEX idx_jobs_status_created_at IS 'Optimises dashboard status filtering';
COMMENT ON INDEX idx_tasks_status_priority IS 'Partial index for pending task queue, optimises worker claims';
