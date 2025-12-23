-- Migration: Enable RLS on schedulers and job_share_links tables
-- Fixes security advisory warnings for unrestricted table access

-- =============================================================================
-- SCHEDULERS TABLE
-- =============================================================================

-- Enable RLS
ALTER TABLE schedulers ENABLE ROW LEVEL SECURITY;

-- Users can view schedulers for their organisation
CREATE POLICY "Users can view own org schedulers"
ON schedulers FOR SELECT
USING (
    organisation_id IN (
        SELECT organisation_id FROM users WHERE id = auth.uid()
    )
);

-- Users can create schedulers for their organisation
CREATE POLICY "Users can create own org schedulers"
ON schedulers FOR INSERT
WITH CHECK (
    organisation_id IN (
        SELECT organisation_id FROM users WHERE id = auth.uid()
    )
);

-- Users can update schedulers for their organisation
CREATE POLICY "Users can update own org schedulers"
ON schedulers FOR UPDATE
USING (
    organisation_id IN (
        SELECT organisation_id FROM users WHERE id = auth.uid()
    )
);

-- Users can delete schedulers for their organisation
CREATE POLICY "Users can delete own org schedulers"
ON schedulers FOR DELETE
USING (
    organisation_id IN (
        SELECT organisation_id FROM users WHERE id = auth.uid()
    )
);

-- =============================================================================
-- JOB_SHARE_LINKS TABLE
-- =============================================================================

-- Enable RLS
ALTER TABLE job_share_links ENABLE ROW LEVEL SECURITY;

-- Authenticated users can view share links for jobs in their organisation
CREATE POLICY "Users can view own org share links"
ON job_share_links FOR SELECT
USING (
    job_id IN (
        SELECT id FROM jobs
        WHERE organisation_id IN (
            SELECT organisation_id FROM users WHERE id = auth.uid()
        )
    )
);

-- Authenticated users can create share links for jobs in their organisation
CREATE POLICY "Users can create share links for own org jobs"
ON job_share_links FOR INSERT
WITH CHECK (
    job_id IN (
        SELECT id FROM jobs
        WHERE organisation_id IN (
            SELECT organisation_id FROM users WHERE id = auth.uid()
        )
    )
);

-- Authenticated users can update (revoke) share links for jobs in their organisation
CREATE POLICY "Users can update own org share links"
ON job_share_links FOR UPDATE
USING (
    job_id IN (
        SELECT id FROM jobs
        WHERE organisation_id IN (
            SELECT organisation_id FROM users WHERE id = auth.uid()
        )
    )
);

-- Authenticated users can delete share links for jobs in their organisation
CREATE POLICY "Users can delete own org share links"
ON job_share_links FOR DELETE
USING (
    job_id IN (
        SELECT id FROM jobs
        WHERE organisation_id IN (
            SELECT organisation_id FROM users WHERE id = auth.uid()
        )
    )
);

-- Anonymous users can view share links by token (for public job viewing)
-- This allows the share link lookup without authentication
CREATE POLICY "Anonymous can view share links by token"
ON job_share_links FOR SELECT
TO anon
USING (
    revoked_at IS NULL
    AND (expires_at IS NULL OR expires_at > NOW())
);
