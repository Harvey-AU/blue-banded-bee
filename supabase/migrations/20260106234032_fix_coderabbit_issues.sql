-- Fix issues identified by CodeRabbit review
-- 1. RLS policy - Add SECURITY DEFINER to increment_daily_usage
-- 2. Timezone - Fix next_midnight_utc to use explicit UTC handling
-- 3. Local variable - Use v_max_to_promote instead of mutating p_limit
-- 4. Fix organisation_quota_status view to use UTC dates

-- =============================================================================
-- FIX 1: Timezone-correct next_midnight_utc function
-- =============================================================================
-- CURRENT_DATE is session-timezone dependent. Use explicit UTC handling.
CREATE OR REPLACE FUNCTION next_midnight_utc()
RETURNS TIMESTAMPTZ
LANGUAGE sql
STABLE
AS $$
    -- Get current time in UTC, truncate to day, add 1 day for next midnight
    SELECT date_trunc('day', NOW() AT TIME ZONE 'UTC' + INTERVAL '1 day') AT TIME ZONE 'UTC';
$$;

COMMENT ON FUNCTION next_midnight_utc IS
'Returns the next midnight UTC timestamp. Uses explicit UTC handling to avoid session timezone issues.';

-- =============================================================================
-- FIX 2: Add SECURITY DEFINER to increment_daily_usage
-- =============================================================================
-- The RLS policy checks auth.jwt() which is NULL for DATABASE_URL connections.
-- SECURITY DEFINER allows the function to bypass RLS as the function owner.
CREATE OR REPLACE FUNCTION increment_daily_usage(p_org_id UUID, p_pages INTEGER DEFAULT 1)
RETURNS VOID
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    v_limit INTEGER;
    v_new_usage INTEGER;
BEGIN
    -- Upsert daily usage
    INSERT INTO daily_usage (organisation_id, usage_date, pages_processed, updated_at)
    VALUES (p_org_id, (NOW() AT TIME ZONE 'UTC')::DATE, p_pages, NOW())
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
Uses SECURITY DEFINER to bypass RLS when called from application code.';

-- =============================================================================
-- FIX 3: Use local variable instead of mutating p_limit parameter
-- =============================================================================
CREATE OR REPLACE FUNCTION promote_waiting_tasks_for_org(p_org_id UUID, p_limit INTEGER DEFAULT 100)
RETURNS INTEGER
LANGUAGE plpgsql
AS $$
DECLARE
    v_promoted INTEGER := 0;
    v_quota_remaining INTEGER;
    v_task_id UUID;
    v_max_to_promote INTEGER;  -- Local variable instead of mutating p_limit
BEGIN
    -- Get current quota
    v_quota_remaining := get_daily_quota_remaining(p_org_id);

    -- Determine actual limit based on quota (use local variable)
    v_max_to_promote := LEAST(p_limit, v_quota_remaining);

    IF v_max_to_promote <= 0 THEN
        RETURN 0;
    END IF;

    -- Promote up to v_max_to_promote tasks across all running jobs for this org
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
        LIMIT v_max_to_promote
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
-- FIX 4: Update view to use UTC-correct date calculations
-- =============================================================================
DROP VIEW IF EXISTS organisation_quota_status;
CREATE VIEW organisation_quota_status AS
SELECT
    o.id AS organisation_id,
    o.name AS organisation_name,
    p.name AS plan_name,
    p.display_name AS plan_display_name,
    p.daily_page_limit,
    COALESCE(du.pages_processed, 0) AS pages_used_today,
    GREATEST(0, p.daily_page_limit - COALESCE(du.pages_processed, 0)) AS pages_remaining_today,
    ROUND(COALESCE(du.pages_processed, 0)::NUMERIC / NULLIF(p.daily_page_limit, 0) * 100, 1) AS usage_percentage,
    next_midnight_utc() AS resets_at
FROM organisations o
JOIN plans p ON o.plan_id = p.id
LEFT JOIN daily_usage du ON du.organisation_id = o.id
    AND du.usage_date = (NOW() AT TIME ZONE 'UTC')::DATE;

COMMENT ON VIEW organisation_quota_status IS
'Current quota status for all organisations. Uses UTC dates for consistency.';

-- =============================================================================
-- FIX 5: Update get_daily_quota_remaining to use UTC date
-- =============================================================================
CREATE OR REPLACE FUNCTION get_daily_quota_remaining(p_org_id UUID)
RETURNS INTEGER
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
    v_limit INTEGER;
    v_used INTEGER;
BEGIN
    -- Get the org's plan limit
    SELECT p.daily_page_limit INTO v_limit
    FROM organisations o
    JOIN plans p ON o.plan_id = p.id
    WHERE o.id = p_org_id;

    IF v_limit IS NULL THEN
        -- No plan found, default to free plan limit
        SELECT daily_page_limit INTO v_limit
        FROM plans
        WHERE name = 'free'
        LIMIT 1;

        -- If still null (no free plan exists), allow unlimited
        IF v_limit IS NULL THEN
            RETURN 999999;
        END IF;
    END IF;

    -- Get today's usage (UTC date)
    SELECT COALESCE(pages_processed, 0) INTO v_used
    FROM daily_usage
    WHERE organisation_id = p_org_id
      AND usage_date = (NOW() AT TIME ZONE 'UTC')::DATE;

    IF v_used IS NULL THEN
        v_used := 0;
    END IF;

    RETURN GREATEST(0, v_limit - v_used);
END;
$$;

COMMENT ON FUNCTION get_daily_quota_remaining IS
'Returns the number of pages remaining in the organisation''s daily quota.
Uses UTC date for consistency. Defaults to free plan limit if org has no plan assigned.';

-- =============================================================================
-- FIX 6: Update get_organisation_usage_stats to use UTC date
-- =============================================================================
CREATE OR REPLACE FUNCTION get_organisation_usage_stats(p_org_id UUID)
RETURNS TABLE(
    daily_limit INTEGER,
    daily_used INTEGER,
    daily_remaining INTEGER,
    plan_name TEXT,
    plan_display_name TEXT,
    reset_time TIMESTAMPTZ
)
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
    v_limit INTEGER;
    v_used INTEGER;
    v_plan_name TEXT;
    v_plan_display TEXT;
BEGIN
    -- Get the org's plan details
    SELECT p.daily_page_limit, p.name, p.display_name
    INTO v_limit, v_plan_name, v_plan_display
    FROM organisations o
    JOIN plans p ON o.plan_id = p.id
    WHERE o.id = p_org_id;

    IF v_limit IS NULL THEN
        v_limit := 0;
        v_plan_name := 'none';
        v_plan_display := 'No Plan';
    END IF;

    -- Get today's usage (UTC date)
    SELECT COALESCE(du.pages_processed, 0) INTO v_used
    FROM daily_usage du
    WHERE du.organisation_id = p_org_id
      AND du.usage_date = (NOW() AT TIME ZONE 'UTC')::DATE;

    IF v_used IS NULL THEN
        v_used := 0;
    END IF;

    -- Return stats
    daily_limit := v_limit;
    daily_used := v_used;
    daily_remaining := GREATEST(0, v_limit - v_used);
    plan_name := v_plan_name;
    plan_display_name := v_plan_display;
    reset_time := next_midnight_utc();

    RETURN NEXT;
END;
$$;

COMMENT ON FUNCTION get_organisation_usage_stats IS
'Returns comprehensive usage statistics for dashboard display.
Uses UTC dates for consistency. Includes current usage, limits, plan info, and next reset time.';
