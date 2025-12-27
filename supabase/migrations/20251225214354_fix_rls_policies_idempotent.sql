-- Migration: Recreate RLS policies idempotently
-- Fixes reset db failure where policies already exist
-- Adds WITH CHECK to UPDATE policies for security
-- Uses EXISTS for better performance on job_share_links

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
USING (organisation_id = public.user_organisation_id())
WITH CHECK (organisation_id = public.user_organisation_id());

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

-- Recreate policies using EXISTS for better performance
CREATE POLICY "Users can view own org share links"
ON job_share_links FOR SELECT
USING (
    EXISTS (
        SELECT 1 FROM jobs
        WHERE jobs.id = job_share_links.job_id
        AND jobs.organisation_id = public.user_organisation_id()
    )
);

CREATE POLICY "Users can create share links for own org jobs"
ON job_share_links FOR INSERT
WITH CHECK (
    EXISTS (
        SELECT 1 FROM jobs
        WHERE jobs.id = job_share_links.job_id
        AND jobs.organisation_id = public.user_organisation_id()
    )
);

CREATE POLICY "Users can update own org share links"
ON job_share_links FOR UPDATE
USING (
    EXISTS (
        SELECT 1 FROM jobs
        WHERE jobs.id = job_share_links.job_id
        AND jobs.organisation_id = public.user_organisation_id()
    )
)
WITH CHECK (
    EXISTS (
        SELECT 1 FROM jobs
        WHERE jobs.id = job_share_links.job_id
        AND jobs.organisation_id = public.user_organisation_id()
    )
);

CREATE POLICY "Users can delete own org share links"
ON job_share_links FOR DELETE
USING (
    EXISTS (
        SELECT 1 FROM jobs
        WHERE jobs.id = job_share_links.job_id
        AND jobs.organisation_id = public.user_organisation_id()
    )
);
