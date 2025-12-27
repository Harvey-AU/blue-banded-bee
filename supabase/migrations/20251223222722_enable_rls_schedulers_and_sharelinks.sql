-- Migration: Enable RLS on schedulers and job_share_links tables
-- Fixes security advisory warnings for unrestricted table access

-- =============================================================================
-- HELPER FUNCTION
-- =============================================================================

-- Security definer function to get current user's organisation ID
-- Simplifies RLS policies and improves readability
CREATE OR REPLACE FUNCTION public.user_organisation_id()
RETURNS uuid AS $$
  SELECT organisation_id FROM users WHERE id = auth.uid()
$$ LANGUAGE sql STABLE SECURITY DEFINER;

-- =============================================================================
-- SCHEDULERS TABLE
-- =============================================================================

-- Enable RLS
ALTER TABLE schedulers ENABLE ROW LEVEL SECURITY;

-- Drop existing policies if they exist (for idempotency)
DROP POLICY IF EXISTS "Users can view own org schedulers" ON schedulers;
DROP POLICY IF EXISTS "Users can create own org schedulers" ON schedulers;
DROP POLICY IF EXISTS "Users can update own org schedulers" ON schedulers;
DROP POLICY IF EXISTS "Users can delete own org schedulers" ON schedulers;

-- Users can view schedulers for their organisation
CREATE POLICY "Users can view own org schedulers"
ON schedulers FOR SELECT
USING (organisation_id = public.user_organisation_id());

-- Users can create schedulers for their organisation
CREATE POLICY "Users can create own org schedulers"
ON schedulers FOR INSERT
WITH CHECK (organisation_id = public.user_organisation_id());

-- Users can update schedulers for their organisation
CREATE POLICY "Users can update own org schedulers"
ON schedulers FOR UPDATE
USING (organisation_id = public.user_organisation_id());

-- Users can delete schedulers for their organisation
CREATE POLICY "Users can delete own org schedulers"
ON schedulers FOR DELETE
USING (organisation_id = public.user_organisation_id());

-- =============================================================================
-- JOB_SHARE_LINKS TABLE
-- =============================================================================

-- Enable RLS
ALTER TABLE job_share_links ENABLE ROW LEVEL SECURITY;

-- Drop existing policies if they exist (for idempotency)
DROP POLICY IF EXISTS "Users can view own org share links" ON job_share_links;
DROP POLICY IF EXISTS "Users can create share links for own org jobs" ON job_share_links;
DROP POLICY IF EXISTS "Users can update own org share links" ON job_share_links;
DROP POLICY IF EXISTS "Users can delete own org share links" ON job_share_links;

-- Authenticated users can view share links for jobs in their organisation
CREATE POLICY "Users can view own org share links"
ON job_share_links FOR SELECT
USING (
    job_id IN (
        SELECT id FROM jobs WHERE organisation_id = public.user_organisation_id()
    )
);

-- Authenticated users can create share links for jobs in their organisation
CREATE POLICY "Users can create share links for own org jobs"
ON job_share_links FOR INSERT
WITH CHECK (
    job_id IN (
        SELECT id FROM jobs WHERE organisation_id = public.user_organisation_id()
    )
);

-- Authenticated users can update (revoke) share links for jobs in their organisation
CREATE POLICY "Users can update own org share links"
ON job_share_links FOR UPDATE
USING (
    job_id IN (
        SELECT id FROM jobs WHERE organisation_id = public.user_organisation_id()
    )
);

-- Authenticated users can delete share links for jobs in their organisation
CREATE POLICY "Users can delete own org share links"
ON job_share_links FOR DELETE
USING (
    job_id IN (
        SELECT id FROM jobs WHERE organisation_id = public.user_organisation_id()
    )
);

-- NOTE: No anonymous SELECT policy for job_share_links.
-- Token validation is handled by the Go backend using service role,
-- which bypasses RLS. This prevents token enumeration attacks.
