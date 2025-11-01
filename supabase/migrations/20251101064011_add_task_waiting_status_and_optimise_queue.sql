-- Migration: Add 'waiting' status and optimise task queue performance
--
-- Problem: GetNextTask CTE query scans thousands of 'pending' tasks whose jobs
-- are at their concurrency limit, causing 5ms-107s query times under load.
--
-- Solution: Introduce 'waiting' status for tasks blocked by job concurrency.
-- Only 'pending' tasks are eligible for claiming, dramatically reducing scan size.
--
-- Performance impact: Reduces typical claim query from scanning 5,000 rows to <100 rows.

-- Step 1: Add check constraint for valid status values (status is TEXT, not ENUM)
-- This ensures only valid status values can be inserted ('waiting' is now allowed)
DO $$
BEGIN
  -- Drop existing constraint if it exists
  ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_status_check;

  -- Add new constraint including 'waiting' status
  ALTER TABLE tasks ADD CONSTRAINT tasks_status_check
    CHECK (status IN ('pending', 'running', 'completed', 'failed', 'skipped', 'waiting'));
END $$;

-- Step 2: Create optimised partial index for ready-to-claim tasks
-- This index only includes tasks that are actually claimable (status='pending')
-- Ordered by priority_score DESC, created_at for optimal claim ordering
-- Note: Cannot use CONCURRENTLY in Supabase migrations (runs in transaction)
CREATE INDEX IF NOT EXISTS idx_tasks_pending_ready
  ON tasks (priority_score DESC, created_at, job_id)
  WHERE status = 'pending';

-- Step 3: Create index for waiting tasks per job
-- When a job frees capacity, we need to quickly find waiting tasks for that job
CREATE INDEX IF NOT EXISTS idx_tasks_waiting_by_job
  ON tasks (job_id, priority_score DESC, created_at)
  WHERE status = 'waiting';

-- Step 4: Add comment explaining the status state machine
COMMENT ON COLUMN tasks.status IS
'Task lifecycle states:
- pending: Ready to be claimed by workers (job has capacity)
- waiting: Queued, blocked by job concurrency limit
- running: Currently being processed by a worker
- completed: Successfully finished
- failed: Permanently failed after retries
- skipped: Excluded from processing (e.g., max_pages limit)

State transitions:
- pending -> running (worker claims task)
- running -> completed/failed (task finishes)
- running -> pending (task retry)
- waiting -> pending (job frees capacity)
- pending -> waiting (job reaches concurrency limit during enqueue)';

-- Step 5: Create function to transition waiting tasks to pending
-- Called when a task completes to free up a job concurrency slot
CREATE OR REPLACE FUNCTION promote_waiting_task_for_job(p_job_id UUID)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
  -- Move highest priority waiting task to pending for this job
  -- Only promote if job has capacity
  UPDATE tasks
  SET status = 'pending'
  WHERE id = (
    SELECT t.id
    FROM tasks t
    INNER JOIN jobs j ON t.job_id = j.id
    WHERE t.job_id = p_job_id
      AND t.status = 'waiting'
      AND j.status = 'running'
      AND (j.concurrency IS NULL OR j.concurrency = 0 OR j.running_tasks < j.concurrency)
    ORDER BY t.priority_score DESC, t.created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
  );
END;
$$;

COMMENT ON FUNCTION promote_waiting_task_for_job IS
'Promotes one waiting task to pending status when a job frees capacity.
Called automatically when tasks complete to maintain optimal queue depth.
Uses FOR UPDATE SKIP LOCKED to avoid contention between concurrent promotions.';

-- Step 6: Create helper function to check if job can accept more tasks
CREATE OR REPLACE FUNCTION job_has_capacity(p_job_id UUID)
RETURNS BOOLEAN
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
  v_has_capacity BOOLEAN;
BEGIN
  SELECT (j.concurrency IS NULL OR j.concurrency = 0 OR j.running_tasks < j.concurrency)
    INTO v_has_capacity
  FROM jobs j
  WHERE j.id = p_job_id
    AND j.status = 'running';

  RETURN COALESCE(v_has_capacity, FALSE);
END;
$$;

COMMENT ON FUNCTION job_has_capacity IS
'Returns true if the job can accept more running tasks based on its concurrency limit.
Used during task enqueueing to determine initial status (pending vs waiting).';

-- Step 7: Backfill existing pending tasks to waiting status
-- Move pending tasks to waiting if their job is at or over concurrency limit
-- This ensures the new index is immediately effective
UPDATE tasks t
SET status = 'waiting'
FROM jobs j
WHERE t.job_id = j.id
  AND t.status = 'pending'
  AND j.status = 'running'
  AND j.concurrency IS NOT NULL
  AND j.concurrency > 0
  AND j.running_tasks >= j.concurrency;
