-- Fix queue helper functions to accept TEXT job IDs, matching the existing schema

-- Remove legacy UUID-typed definitions so the new TEXT versions are authoritative
DROP FUNCTION IF EXISTS promote_waiting_task_for_job(UUID);
DROP FUNCTION IF EXISTS job_has_capacity(UUID);

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

COMMENT ON FUNCTION promote_waiting_task_for_job(TEXT) IS
'Promotes one waiting task to pending status when a job frees capacity.
Called automatically when tasks complete to maintain optimal queue depth.
Uses FOR UPDATE SKIP LOCKED to avoid contention between concurrent promotions.';

CREATE OR REPLACE FUNCTION job_has_capacity(p_job_id TEXT)
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

COMMENT ON FUNCTION job_has_capacity(TEXT) IS
'Returns true if the job can accept more running tasks based on its concurrency limit.
Used during task enqueueing to determine initial status (pending vs waiting).';
