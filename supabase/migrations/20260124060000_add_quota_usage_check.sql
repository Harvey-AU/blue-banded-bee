-- Add function to check if org has exceeded daily quota based on COMPLETED pages only
-- This is used by GetNextTask as the last line of defence against over-processing
-- Different from get_daily_quota_remaining which counts pending+running tasks

CREATE OR REPLACE FUNCTION is_org_over_daily_quota(p_org_id UUID)
RETURNS BOOLEAN
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

        -- If still null (no free plan exists), not over quota
        IF v_limit IS NULL THEN
            RETURN FALSE;
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

    -- Return true if pages_processed >= limit
    RETURN v_used >= v_limit;
END;
$$;

COMMENT ON FUNCTION is_org_over_daily_quota IS
'Returns TRUE if the organisation has processed >= their daily limit.
Only counts completed pages (pages_processed), not pending/running tasks.
Used by GetNextTask as the last line of defence against over-processing.';
