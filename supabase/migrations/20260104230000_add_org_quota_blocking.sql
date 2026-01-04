-- Migration: Add org-level quota blocking for efficient worker scaling
--
-- Purpose: Track when an organisation's quota is exhausted at the org level,
-- so workers can skip all jobs for that org without per-job queries.
-- When quota resets (midnight UTC), a background process clears the block
-- and wakes workers.
--
-- Key changes:
-- 1. Add quota_exhausted_until column to organisations
-- 2. Update increment_daily_usage to set org blocked when quota hits 0
-- 3. Create function to clear expired blocks (called by Go task monitor)
-- 4. Update promote_waiting_task_for_job to set org blocked flag

-- =============================================================================
-- STEP 1: Add quota_exhausted_until to organisations
-- =============================================================================
ALTER TABLE organisations
ADD COLUMN IF NOT EXISTS quota_exhausted_until TIMESTAMPTZ;

COMMENT ON COLUMN organisations.quota_exhausted_until IS
'When set, indicates the org has exhausted their daily quota until this timestamp.
NULL means quota is available. Workers skip jobs for blocked orgs.
Typically set to next midnight UTC. Cleared by periodic quota reset check.';

-- Index for efficient blocked org lookups
CREATE INDEX IF NOT EXISTS idx_organisations_quota_blocked
ON organisations(quota_exhausted_until)
WHERE quota_exhausted_until IS NOT NULL;

-- =============================================================================
-- STEP 2: Helper function to calculate next midnight UTC
-- =============================================================================
CREATE OR REPLACE FUNCTION next_midnight_utc()
RETURNS TIMESTAMPTZ
LANGUAGE sql
STABLE
AS $$
    SELECT (CURRENT_DATE + INTERVAL '1 day')::TIMESTAMPTZ;
$$;

COMMENT ON FUNCTION next_midnight_utc IS
'Returns the next midnight UTC timestamp. Used for setting quota_exhausted_until.';

-- =============================================================================
-- STEP 3: Update increment_daily_usage to set org blocked when quota hits 0
-- =============================================================================
CREATE OR REPLACE FUNCTION increment_daily_usage(p_org_id UUID, p_pages INTEGER DEFAULT 1)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    v_limit INTEGER;
    v_new_usage INTEGER;
BEGIN
    -- Upsert daily usage
    INSERT INTO daily_usage (organisation_id, usage_date, pages_processed, updated_at)
    VALUES (p_org_id, CURRENT_DATE, p_pages, NOW())
    ON CONFLICT (organisation_id, usage_date)
    DO UPDATE SET
        pages_processed = daily_usage.pages_processed + p_pages,
        updated_at = NOW()
    RETURNING pages_processed INTO v_new_usage;

    -- Get the org's plan limit
    SELECT p.daily_page_limit INTO v_limit
    FROM organisations o
    JOIN plans p ON o.plan_id = p.id
    WHERE o.id = p_org_id;

    -- If usage has hit or exceeded limit, set quota_exhausted_until
    IF v_limit IS NOT NULL AND v_new_usage >= v_limit THEN
        UPDATE organisations
        SET quota_exhausted_until = next_midnight_utc()
        WHERE id = p_org_id
          AND quota_exhausted_until IS NULL;  -- Only set if not already set
    END IF;
END;
$$;

COMMENT ON FUNCTION increment_daily_usage IS
'Atomically increments the daily page usage counter for an organisation.
When usage hits the plan limit, sets quota_exhausted_until to block further processing.';

-- =============================================================================
-- STEP 4: Update promote_waiting_task_for_job to also set blocked flag
-- =============================================================================
CREATE OR REPLACE FUNCTION promote_waiting_task_for_job(p_job_id UUID)
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
        -- Fall through to original logic
    ELSE
        -- Check quota
        v_quota_remaining := get_daily_quota_remaining(v_org_id);

        IF v_quota_remaining <= 0 THEN
            -- Quota exhausted, ensure org is marked as blocked
            UPDATE organisations
            SET quota_exhausted_until = next_midnight_utc()
            WHERE id = v_org_id
              AND quota_exhausted_until IS NULL;
            -- Don't promote
            RETURN;
        END IF;
    END IF;

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
    )
    RETURNING id INTO v_task_id;

    -- If we promoted a task, increment usage (which may set blocked flag)
    IF v_task_id IS NOT NULL AND v_org_id IS NOT NULL THEN
        PERFORM increment_daily_usage(v_org_id, 1);
    END IF;
END;
$$;

COMMENT ON FUNCTION promote_waiting_task_for_job IS
'Promotes one waiting task to pending status when a job frees capacity.
Checks organisation daily quota before promotion and sets blocked flag if exhausted.
Increments daily usage counter when a task is promoted.';

-- =============================================================================
-- STEP 5: Function to clear expired quota blocks (called by Go task monitor)
-- =============================================================================
CREATE OR REPLACE FUNCTION clear_expired_quota_blocks()
RETURNS TABLE(organisation_id UUID)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    UPDATE organisations
    SET quota_exhausted_until = NULL
    WHERE quota_exhausted_until IS NOT NULL
      AND quota_exhausted_until < NOW()
    RETURNING id AS organisation_id;
END;
$$;

COMMENT ON FUNCTION clear_expired_quota_blocks IS
'Clears quota_exhausted_until for orgs where the block has expired.
Returns the IDs of orgs that were unblocked, so workers can be notified.
Should be called periodically by the task monitor (e.g., every 30 seconds).';

-- =============================================================================
-- STEP 6: Function to promote waiting tasks for unblocked orgs
-- =============================================================================
CREATE OR REPLACE FUNCTION promote_waiting_tasks_for_org(p_org_id UUID, p_limit INTEGER DEFAULT 100)
RETURNS INTEGER
LANGUAGE plpgsql
AS $$
DECLARE
    v_promoted INTEGER := 0;
    v_quota_remaining INTEGER;
    v_task_id UUID;
BEGIN
    -- Get current quota
    v_quota_remaining := get_daily_quota_remaining(p_org_id);

    -- Limit promotions to available quota
    IF v_quota_remaining < p_limit THEN
        p_limit := v_quota_remaining;
    END IF;

    IF p_limit <= 0 THEN
        RETURN 0;
    END IF;

    -- Promote up to p_limit tasks across all running jobs for this org
    FOR v_task_id IN
        SELECT t.id
        FROM tasks t
        INNER JOIN jobs j ON t.job_id = j.id
        WHERE j.organisation_id = p_org_id
          AND t.status = 'waiting'
          AND j.status = 'running'
          AND (j.concurrency IS NULL OR j.concurrency = 0 OR
               j.running_tasks + j.pending_tasks < j.concurrency)
        ORDER BY t.priority_score DESC, t.created_at ASC
        LIMIT p_limit
        FOR UPDATE OF t SKIP LOCKED
    LOOP
        UPDATE tasks SET status = 'pending' WHERE id = v_task_id;
        v_promoted := v_promoted + 1;
    END LOOP;

    -- Increment usage for promoted tasks
    IF v_promoted > 0 THEN
        PERFORM increment_daily_usage(p_org_id, v_promoted);
    END IF;

    RETURN v_promoted;
END;
$$;

COMMENT ON FUNCTION promote_waiting_tasks_for_org IS
'Promotes waiting tasks to pending for an organisation after quota resets.
Respects both quota limits and job concurrency limits.
Returns the number of tasks promoted.';

-- =============================================================================
-- STEP 7: Function to check if an org is quota-blocked
-- =============================================================================
CREATE OR REPLACE FUNCTION is_org_quota_blocked(p_org_id UUID)
RETURNS BOOLEAN
LANGUAGE sql
STABLE
AS $$
    SELECT EXISTS (
        SELECT 1 FROM organisations
        WHERE id = p_org_id
          AND quota_exhausted_until IS NOT NULL
          AND quota_exhausted_until >= NOW()
    );
$$;

COMMENT ON FUNCTION is_org_quota_blocked IS
'Returns TRUE if the organisation is currently quota-blocked.
Used by workers and scaling calculations to skip blocked orgs.';

-- =============================================================================
-- STEP 8: Update the organisation_quota_status view
-- =============================================================================
CREATE OR REPLACE VIEW organisation_quota_status AS
SELECT
    o.id AS organisation_id,
    o.name AS organisation_name,
    p.name AS plan_name,
    p.display_name AS plan_display_name,
    p.daily_page_limit,
    COALESCE(du.pages_processed, 0) AS pages_used_today,
    GREATEST(0, p.daily_page_limit - COALESCE(du.pages_processed, 0)) AS pages_remaining_today,
    ROUND(COALESCE(du.pages_processed, 0)::NUMERIC / NULLIF(p.daily_page_limit, 0) * 100, 1) AS usage_percentage,
    o.quota_exhausted_until,
    o.quota_exhausted_until IS NOT NULL AND o.quota_exhausted_until >= NOW() AS is_blocked,
    (CURRENT_DATE + INTERVAL '1 day')::TIMESTAMPTZ AS resets_at
FROM organisations o
JOIN plans p ON o.plan_id = p.id
LEFT JOIN daily_usage du ON du.organisation_id = o.id AND du.usage_date = CURRENT_DATE;

COMMENT ON VIEW organisation_quota_status IS
'Current quota status for all organisations including blocking status. Useful for admin monitoring.';
