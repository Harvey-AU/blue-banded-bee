-- Re-add running_tasks column that was accidentally lost
-- in 20251028005214_convert_all_timestamps_to_timestamptz.sql
-- when the migration dropped and recreated generated columns

-- Add the column with default value 0
ALTER TABLE jobs
ADD COLUMN IF NOT EXISTS running_tasks INTEGER NOT NULL DEFAULT 0;

-- Add check constraint to ensure running_tasks is never negative
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'jobs_running_tasks_non_negative'
  ) THEN
    ALTER TABLE jobs
    ADD CONSTRAINT jobs_running_tasks_non_negative
    CHECK (running_tasks >= 0);
  END IF;
END $$;

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
CREATE INDEX IF NOT EXISTS idx_jobs_running_tasks ON jobs(running_tasks)
WHERE status = 'running';

-- Add comment explaining the column
COMMENT ON COLUMN jobs.running_tasks IS
'Number of tasks currently being processed (claimed but not yet completed/failed). Used to enforce per-job concurrency limits.';
