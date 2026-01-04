-- Migration: Add plans table and daily usage quota system
--
-- Purpose: Implement tiered daily page limits per organisation.
-- Tasks stay in 'waiting' status when daily quota is exhausted,
-- resuming automatically when quota resets at midnight UTC.
--
-- Integration points:
-- 1. promote_waiting_task_for_job() - checks quota before promotion
-- 2. EnqueueURLs() - factors quota into available slots calculation
-- 3. Task completion trigger - increments daily usage counter

-- =============================================================================
-- STEP 1: Create plans table
-- =============================================================================
CREATE TABLE IF NOT EXISTS plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,              -- 'free', 'starter', 'pro', 'business', 'enterprise'
    display_name TEXT NOT NULL,             -- 'Free', 'Starter', etc.
    daily_page_limit INTEGER NOT NULL,      -- Pages per day (1000, 2000, 5000, etc.)
    monthly_price_cents INTEGER NOT NULL,   -- Price in cents (0, 5000, 8000, etc.)
    max_concurrent_jobs INTEGER DEFAULT 3,  -- Max simultaneous active jobs
    max_pages_per_job INTEGER DEFAULT 5000, -- Max pages in a single job
    features JSONB DEFAULT '{}',            -- Future: feature flags
    is_active BOOLEAN DEFAULT TRUE,         -- Can new orgs subscribe to this plan?
    sort_order INTEGER DEFAULT 0,           -- Display ordering
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

COMMENT ON TABLE plans IS
'Subscription tiers defining daily page limits and pricing.
Used by Paddle integration for subscription management.';

COMMENT ON COLUMN plans.daily_page_limit IS
'Maximum pages that can be processed (moved to pending) per day per organisation.
Resets at midnight UTC.';

-- Seed default plans
INSERT INTO plans (name, display_name, daily_page_limit, monthly_price_cents, max_concurrent_jobs, max_pages_per_job, sort_order) VALUES
    ('free', 'Free', 1000, 0, 1, 1000, 0),
    ('starter', 'Starter', 2000, 5000, 3, 2000, 10),
    ('pro', 'Pro', 5000, 8000, 5, 5000, 20),
    ('business', 'Business', 10000, 15000, 10, 10000, 30),
    ('enterprise', 'Enterprise', 100000, 40000, 25, 50000, 40)
ON CONFLICT (name) DO NOTHING;

-- =============================================================================
-- STEP 2: Add plan_id to organisations
-- =============================================================================
ALTER TABLE organisations
ADD COLUMN IF NOT EXISTS plan_id UUID REFERENCES plans(id);

-- Default all existing organisations to free plan
UPDATE organisations
SET plan_id = (SELECT id FROM plans WHERE name = 'free')
WHERE plan_id IS NULL;

-- Create function to get free plan ID (required because DEFAULT cannot use subquery)
CREATE OR REPLACE FUNCTION get_free_plan_id()
RETURNS UUID AS $$
    SELECT id FROM plans WHERE name = 'free' LIMIT 1;
$$ LANGUAGE SQL STABLE;

-- Make plan_id NOT NULL after backfill (with default for new orgs)
ALTER TABLE organisations
ALTER COLUMN plan_id SET DEFAULT get_free_plan_id();

-- Add index for plan lookups
CREATE INDEX IF NOT EXISTS idx_organisations_plan_id ON organisations(plan_id);

-- =============================================================================
-- STEP 3: Create daily usage tracking table
-- =============================================================================
CREATE TABLE IF NOT EXISTS daily_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
    usage_date DATE NOT NULL,               -- The day (UTC) this usage applies to
    pages_processed INTEGER DEFAULT 0,      -- Count of pages moved to pending/completed
    jobs_created INTEGER DEFAULT 0,         -- Count of jobs created
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(organisation_id, usage_date)
);

COMMENT ON TABLE daily_usage IS
'Tracks daily page usage per organisation for quota enforcement.
One row per org per day. Quota resets at midnight UTC.';

-- Efficient lookups for current day usage
CREATE INDEX IF NOT EXISTS idx_daily_usage_org_date
ON daily_usage(organisation_id, usage_date DESC);

-- =============================================================================
-- STEP 4: Create helper functions for quota management
-- =============================================================================

-- Get remaining daily quota for an organisation
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
        -- No plan found, return 0 (blocked)
        RETURN 0;
    END IF;

    -- Get today's usage (UTC)
    SELECT COALESCE(pages_processed, 0) INTO v_used
    FROM daily_usage
    WHERE organisation_id = p_org_id
      AND usage_date = CURRENT_DATE;

    IF v_used IS NULL THEN
        v_used := 0;
    END IF;

    RETURN GREATEST(0, v_limit - v_used);
END;
$$;

COMMENT ON FUNCTION get_daily_quota_remaining IS
'Returns the number of pages remaining in the organisation''s daily quota.
Returns 0 if quota exhausted or no valid plan.';

-- Get full usage stats for an organisation
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

    -- Get today's usage
    SELECT COALESCE(du.pages_processed, 0) INTO v_used
    FROM daily_usage du
    WHERE du.organisation_id = p_org_id
      AND du.usage_date = CURRENT_DATE;

    IF v_used IS NULL THEN
        v_used := 0;
    END IF;

    -- Return stats
    daily_limit := v_limit;
    daily_used := v_used;
    daily_remaining := GREATEST(0, v_limit - v_used);
    plan_name := v_plan_name;
    plan_display_name := v_plan_display;
    -- Next midnight UTC
    reset_time := (CURRENT_DATE + INTERVAL '1 day')::TIMESTAMPTZ;

    RETURN NEXT;
END;
$$;

COMMENT ON FUNCTION get_organisation_usage_stats IS
'Returns comprehensive usage statistics for dashboard display.
Includes current usage, limits, plan info, and next reset time.';

-- Increment daily usage counter
CREATE OR REPLACE FUNCTION increment_daily_usage(p_org_id UUID, p_pages INTEGER DEFAULT 1)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO daily_usage (organisation_id, usage_date, pages_processed, updated_at)
    VALUES (p_org_id, CURRENT_DATE, p_pages, NOW())
    ON CONFLICT (organisation_id, usage_date)
    DO UPDATE SET
        pages_processed = daily_usage.pages_processed + p_pages,
        updated_at = NOW();
END;
$$;

COMMENT ON FUNCTION increment_daily_usage IS
'Atomically increments the daily page usage counter for an organisation.
Called when tasks are promoted from waiting to pending.';

-- =============================================================================
-- STEP 5: Update promote_waiting_task_for_job to check quota
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
            -- Quota exhausted, don't promote
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

    -- If we promoted a task, increment usage
    IF v_task_id IS NOT NULL AND v_org_id IS NOT NULL THEN
        PERFORM increment_daily_usage(v_org_id, 1);
    END IF;
END;
$$;

COMMENT ON FUNCTION promote_waiting_task_for_job IS
'Promotes one waiting task to pending status when a job frees capacity.
Now also checks organisation daily quota before promotion.
Increments daily usage counter when a task is promoted.';

-- =============================================================================
-- STEP 6: Create function to get quota-aware available slots
-- =============================================================================
CREATE OR REPLACE FUNCTION get_quota_available_slots(p_org_id UUID, p_job_concurrency_slots INTEGER)
RETURNS INTEGER
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
    v_quota_remaining INTEGER;
BEGIN
    v_quota_remaining := get_daily_quota_remaining(p_org_id);

    -- Return the minimum of job concurrency slots and quota remaining
    RETURN LEAST(p_job_concurrency_slots, v_quota_remaining);
END;
$$;

COMMENT ON FUNCTION get_quota_available_slots IS
'Returns the effective available slots considering both job concurrency and daily quota.
Used by EnqueueURLs to determine how many tasks can be set to pending.';

-- =============================================================================
-- STEP 7: Enable RLS on new tables
-- =============================================================================
ALTER TABLE plans ENABLE ROW LEVEL SECURITY;
ALTER TABLE daily_usage ENABLE ROW LEVEL SECURITY;

-- Plans are readable by all authenticated users (for pricing page)
CREATE POLICY "Plans are publicly readable" ON plans
    FOR SELECT USING (TRUE);

-- Daily usage readable by org members
CREATE POLICY "Users can view their organisation usage" ON daily_usage
    FOR SELECT USING (
        organisation_id IN (
            SELECT om.organisation_id
            FROM organisation_members om
            WHERE om.user_id = auth.uid()
        )
    );

-- Daily usage is modified by service role only (via functions)
CREATE POLICY "Service role can manage usage" ON daily_usage
    FOR ALL USING (auth.jwt() ->> 'role' = 'service_role');

-- =============================================================================
-- STEP 8: Add useful views for monitoring
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
    (CURRENT_DATE + INTERVAL '1 day')::TIMESTAMPTZ AS resets_at
FROM organisations o
JOIN plans p ON o.plan_id = p.id
LEFT JOIN daily_usage du ON du.organisation_id = o.id AND du.usage_date = CURRENT_DATE;

COMMENT ON VIEW organisation_quota_status IS
'Current quota status for all organisations. Useful for admin monitoring.';
