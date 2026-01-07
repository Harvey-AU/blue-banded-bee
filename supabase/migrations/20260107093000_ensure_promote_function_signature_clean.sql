-- Ensure promote_waiting_task_for_job has clean TEXT signature
-- This explicitly drops any legacy UUID signatures that may exist in preview databases
-- Resolves "commit unexpectedly resulted in rollback" errors (BLUE-BANDED-BEE-7B)

DROP FUNCTION IF EXISTS promote_waiting_task_for_job(UUID);
DROP FUNCTION IF EXISTS promote_waiting_task_for_job(TEXT);

CREATE FUNCTION promote_waiting_task_for_job(p_job_id TEXT)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
  -- Promote highest priority waiting task to pending for this job
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
    FOR UPDATE OF t SKIP LOCKED
  );
END;
$$;

COMMENT ON FUNCTION promote_waiting_task_for_job(TEXT) IS
'Promotes one waiting task to pending status when a job frees capacity.
Uses FOR UPDATE OF t SKIP LOCKED to avoid locking job rows and prevent
deadlocks with concurrent task claims. Accepts TEXT job_id to match jobs.id column type.';
