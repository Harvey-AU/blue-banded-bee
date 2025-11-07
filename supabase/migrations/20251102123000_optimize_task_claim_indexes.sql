-- Optimise task-claim hot path by providing covering indexes
-- 1. Prioritise pending tasks by job to support EXISTS lookups and deterministic ordering
CREATE INDEX IF NOT EXISTS idx_tasks_pending_by_job_priority
  ON tasks (job_id, priority_score DESC, created_at)
  WHERE status = 'pending';

-- 2. Enable index-only lookups for running job capacity checks
CREATE INDEX IF NOT EXISTS idx_jobs_running_capacity
  ON jobs (id)
  INCLUDE (running_tasks, concurrency)
  WHERE status = 'running';
