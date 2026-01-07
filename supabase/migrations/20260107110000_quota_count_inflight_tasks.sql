-- Fix: Quota remaining should account for in-flight tasks (pending + running)
-- This prevents over-queuing when tasks haven't completed yet

CREATE OR REPLACE FUNCTION get_daily_quota_remaining(p_org_id UUID)
RETURNS INTEGER
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
    v_limit INTEGER;
    v_used INTEGER;
    v_in_flight INTEGER;
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

    -- Get today's completed usage (UTC date)
    SELECT COALESCE(pages_processed, 0) INTO v_used
    FROM daily_usage
    WHERE organisation_id = p_org_id
      AND usage_date = (NOW() AT TIME ZONE 'UTC')::DATE;

    IF v_used IS NULL THEN
        v_used := 0;
    END IF;

    -- Count in-flight tasks (pending + running) for this org's jobs
    SELECT COUNT(*) INTO v_in_flight
    FROM tasks t
    JOIN jobs j ON t.job_id = j.id
    WHERE j.organisation_id = p_org_id
      AND t.status IN ('pending', 'running');

    RETURN GREATEST(0, v_limit - v_used - v_in_flight);
END;
$$;

COMMENT ON FUNCTION get_daily_quota_remaining IS
'Returns the number of pages remaining in the organisation''s daily quota.
Accounts for both completed usage and in-flight tasks (pending + running).
Uses UTC date for consistency. Defaults to free plan limit if org has no plan assigned.';
