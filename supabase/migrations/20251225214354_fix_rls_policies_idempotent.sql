-- Migration: Recreate RLS policies idempotently
-- Fixes reset db failure where policies already exist

-- =============================================================================
-- SCHEDULERS TABLE
-- =============================================================================

-- Drop existing policies if they exist
DROP POLICY IF EXISTS "Users can view own org schedulers" ON schedulers;
DROP POLICY IF EXISTS "Users can create own org schedulers" ON schedulers;
DROP POLICY IF EXISTS "Users can update own org schedulers" ON schedulers;
DROP POLICY IF EXISTS "Users can delete own org schedulers" ON schedulers;

-- Recreate policies
CREATE POLICY "Users can view own org schedulers"
ON schedulers FOR SELECT
USING (organisation_id = public.user_organisation_id());

CREATE POLICY "Users can create own org schedulers"
ON schedulers FOR INSERT
WITH CHECK (organisation_id = public.user_organisation_id());

CREATE POLICY "Users can update own org schedulers"
ON schedulers FOR UPDATE
USING (organisation_id = public.user_organisation_id());

CREATE POLICY "Users can delete own org schedulers"
ON schedulers FOR DELETE
USING (organisation_id = public.user_organisation_id());

-- =============================================================================
-- JOB_SHARE_LINKS TABLE
-- =============================================================================

-- Drop existing policies if they exist
DROP POLICY IF EXISTS "Users can view own org share links" ON job_share_links;
DROP POLICY IF EXISTS "Users can create share links for own org jobs" ON job_share_links;
DROP POLICY IF EXISTS "Users can update own org share links" ON job_share_links;
DROP POLICY IF EXISTS "Users can delete own org share links" ON job_share_links;

-- Recreate policies
CREATE POLICY "Users can view own org share links"
ON job_share_links FOR SELECT
USING (
    job_id IN (
        SELECT id FROM jobs WHERE organisation_id = public.user_organisation_id()
    )
);

CREATE POLICY "Users can create share links for own org jobs"
ON job_share_links FOR INSERT
WITH CHECK (
    job_id IN (
        SELECT id FROM jobs WHERE organisation_id = public.user_organisation_id()
    )
);

CREATE POLICY "Users can update own org share links"
ON job_share_links FOR UPDATE
USING (
    job_id IN (
        SELECT id FROM jobs WHERE organisation_id = public.user_organisation_id()
    )
);

CREATE POLICY "Users can delete own org share links"
ON job_share_links FOR DELETE
USING (
    job_id IN (
        SELECT id FROM jobs WHERE organisation_id = public.user_organisation_id()
    )
);
