-- =============================================================================
-- Update get_organisation_usage_stats to include plan_id
-- =============================================================================
DROP FUNCTION IF EXISTS get_organisation_usage_stats(UUID);

CREATE OR REPLACE FUNCTION get_organisation_usage_stats(p_org_id UUID)
RETURNS TABLE(
    daily_limit INTEGER,
    daily_used INTEGER,
    daily_remaining INTEGER,
    plan_id UUID,
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
    v_plan_id UUID;
    v_plan_name TEXT;
    v_plan_display TEXT;
BEGIN
    SELECT o.plan_id, p.daily_page_limit, p.name, p.display_name
    INTO v_plan_id, v_limit, v_plan_name, v_plan_display
    FROM organisations o
    LEFT JOIN plans p ON o.plan_id = p.id
    WHERE o.id = p_org_id;

    IF NOT FOUND THEN
        RETURN;
    END IF;

    IF v_limit IS NULL THEN
        v_limit := 0;
        v_plan_name := 'none';
        v_plan_display := 'No Plan';
    END IF;

    SELECT COALESCE(du.pages_processed, 0) INTO v_used
    FROM daily_usage du
    WHERE du.organisation_id = p_org_id
      AND du.usage_date = (NOW() AT TIME ZONE 'UTC')::DATE;

    IF v_used IS NULL THEN
        v_used := 0;
    END IF;

    daily_limit := v_limit;
    daily_used := v_used;
    daily_remaining := GREATEST(0, v_limit - v_used);
    plan_id := v_plan_id;
    plan_name := v_plan_name;
    plan_display_name := v_plan_display;
    reset_time := next_midnight_utc();

    RETURN NEXT;
END;
$$;
