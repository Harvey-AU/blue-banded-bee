-- Fix promote_waiting_task_for_job to accept TEXT instead of UUID
-- This matches the pattern used by recalculate_job_stats and aligns with
-- jobs.id and tasks.job_id being TEXT columns, not UUID columns.

CREATE OR REPLACE FUNCTION promote_waiting_task_for_job(p_job_id TEXT)
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
  'Promotes the highest priority waiting task to pending for a specific job. '
  'Only promotes if the job has available capacity (based on concurrency limit). '
  'Called by batch processing after task completions to fill freed slots. '
  'Uses TEXT parameter type to match jobs.id column type.';
