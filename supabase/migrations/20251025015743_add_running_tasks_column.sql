-- Add running_tasks column to jobs table
-- This column tracks how many tasks are currently being processed for a job
-- Used to enforce per-job concurrency limits at the database level

-- Add the column with default value 0
ALTER TABLE jobs
ADD COLUMN running_tasks INTEGER NOT NULL DEFAULT 0;

-- Add check constraint to ensure running_tasks is never negative
ALTER TABLE jobs
ADD CONSTRAINT jobs_running_tasks_non_negative
CHECK (running_tasks >= 0);

-- Note: NOT adding jobs_running_tasks_within_total constraint because:
-- 1. Tasks are claimed (running_tasks++) before total_tasks is updated
-- 2. This would cause immediate constraint violations on active jobs
-- 3. The application logic ensures correctness via atomic CTE operations

-- Backfill running_tasks for existing jobs with running tasks
-- This prevents temporary over-claiming until batch updates flush
UPDATE jobs
SET running_tasks = (
    SELECT COUNT(*)
    FROM tasks
    WHERE tasks.job_id = jobs.id
    AND tasks.status = 'running'
)
WHERE status IN ('running', 'pending');

-- Add index for efficient queries filtering by running_tasks
-- This supports the GetNextTask query that checks running_tasks < concurrency
CREATE INDEX idx_jobs_running_tasks ON jobs(running_tasks)
WHERE status = 'running';

-- Add comment explaining the column
COMMENT ON COLUMN jobs.running_tasks IS
'Number of tasks currently being processed (claimed but not yet completed/failed). Used to enforce per-job concurrency limits.';
