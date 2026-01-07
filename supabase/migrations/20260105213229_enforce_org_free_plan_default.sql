-- Enforce that all organisations have a plan (default to free)
-- This ensures orgs are never blocked due to missing plan_id

-- Step 1: Backfill any orgs that somehow have NULL plan_id
UPDATE organisations
SET plan_id = (SELECT id FROM plans WHERE name = 'free' LIMIT 1)
WHERE plan_id IS NULL;

-- Step 2: Add NOT NULL constraint to prevent future nulls
-- The DEFAULT get_free_plan_id() was already set in 20260104212025
ALTER TABLE organisations
ALTER COLUMN plan_id SET NOT NULL;

COMMENT ON COLUMN organisations.plan_id IS
'Reference to the organisation''s subscription plan. Defaults to free plan.
NOT NULL constraint ensures every org has a valid plan for quota enforcement.';
