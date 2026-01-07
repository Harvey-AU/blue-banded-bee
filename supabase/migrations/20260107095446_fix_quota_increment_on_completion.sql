-- Fix: Quota should increment on task COMPLETION, not when tasks become pending
-- Removes increment_daily_usage call from promote_waiting_task_for_job
-- Go code now handles incrementing on completed/failed tasks

DROP FUNCTION IF EXISTS promote_waiting_task_for_job(TEXT);

CREATE FUNCTION promote_waiting_task_for_job(p_job_id TEXT)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    v_org_id UUID;
    v_quota_remaining INTEGER;
    v_task_id UUID;
BEGIN
    -- Get the organisation for this job
    SELECT o.id INTO v_org_id
    FROM jobs j
    JOIN organisations o ON j.organisation_id = o.id
    WHERE j.id = p_job_id;

    IF v_org_id IS NULL THEN
        -- Job has no organisation, allow promotion (legacy behaviour)
        NULL;
    ELSE
        -- Check quota
        v_quota_remaining := get_daily_quota_remaining(v_org_id);

        IF v_quota_remaining <= 0 THEN
            UPDATE organisations
            SET quota_exhausted_until = next_midnight_utc()
            WHERE id = v_org_id
              AND quota_exhausted_until IS NULL;
            RETURN;
        END IF;
    END IF;

    -- Promote highest priority waiting task to pending
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
    )
    RETURNING id INTO v_task_id;

    -- NOTE: Daily usage is NOT incremented here anymore
    -- Usage is incremented when tasks COMPLETE (in Go batch.go)
END;
$$;

COMMENT ON FUNCTION promote_waiting_task_for_job(TEXT) IS
'Promotes one waiting task to pending. Quota increments on task completion, not promotion.';
