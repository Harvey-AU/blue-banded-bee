-- Optimise task-claim hot path
-- Replace unused status/priority index with composite that also filters by job
DROP INDEX IF EXISTS idx_tasks_status_priority;

CREATE INDEX IF NOT EXISTS idx_tasks_job_status_priority_pending
ON tasks (job_id, status, priority_score DESC, created_at ASC)
WHERE status = 'pending';

COMMENT ON INDEX idx_tasks_job_status_priority_pending IS 'Covers worker claim query: job_id filter + status + priority ordering';
