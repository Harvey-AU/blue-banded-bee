-- Convert running_tasks to a generated column to eliminate lock contention
-- This removes the need for workers to lock and increment running_tasks,
-- dramatically reducing database contention during task claiming

-- Drop the existing regular column and constraints
ALTER TABLE jobs
DROP COLUMN IF EXISTS running_tasks CASCADE;

-- Add running_tasks as a generated column that counts running tasks
ALTER TABLE jobs
ADD COLUMN running_tasks INTEGER GENERATED ALWAYS AS (
    (SELECT COUNT(*)::INTEGER
     FROM tasks
     WHERE tasks.job_id = jobs.id
     AND tasks.status = 'running')
) STORED;

-- Add index for performance (on the tasks table for the subquery)
CREATE INDEX IF NOT EXISTS idx_tasks_job_running
ON tasks(job_id, status)
WHERE status = 'running';

-- Update comment
COMMENT ON COLUMN jobs.running_tasks IS
'Auto-calculated count of tasks currently being processed (status = running). Used to enforce per-job concurrency limits without lock contention.';
