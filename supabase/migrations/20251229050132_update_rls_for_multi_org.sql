-- Migration: Update RLS policies to use organisation_members and active_organisation_id
-- Part of platform auth implementation for multi-organisation support
-- IMPORTANT: Deploy this AFTER Go code is updated to use new org context

-- =============================================================================
-- ORGANISATIONS TABLE
-- =============================================================================

DROP POLICY IF EXISTS "Users can access own organisation" ON organisations;
DROP POLICY IF EXISTS "Users can access member organisations" ON organisations;

-- Users can view organisations they are members of
CREATE POLICY "Users can view member organisations"
ON organisations FOR SELECT
USING (
    id IN (SELECT public.user_organisations())
);

-- Users can modify organisations they are members of (INSERT/UPDATE/DELETE)
-- Note: Organisation creation typically happens via service role during registration
CREATE POLICY "Users can modify member organisations"
ON organisations FOR INSERT
WITH CHECK (
    id IN (SELECT public.user_organisations())
);

CREATE POLICY "Users can update member organisations"
ON organisations FOR UPDATE
USING (
    id IN (SELECT public.user_organisations())
)
WITH CHECK (
    id IN (SELECT public.user_organisations())
);

CREATE POLICY "Users can delete member organisations"
ON organisations FOR DELETE
USING (
    id IN (SELECT public.user_organisations())
);

-- =============================================================================
-- JOBS TABLE
-- =============================================================================

DROP POLICY IF EXISTS "Organisation members can access jobs" ON jobs;

-- Users can access jobs for their active organisation only
-- Double-checks both active org match AND membership validation
CREATE POLICY "Users can access active org jobs"
ON jobs FOR ALL
USING (
    organisation_id = public.user_organisation_id()
    AND public.user_is_member_of(organisation_id)
);

-- =============================================================================
-- TASKS TABLE
-- =============================================================================

DROP POLICY IF EXISTS "Organisation members can access tasks" ON tasks;

-- Tasks accessible via job membership
CREATE POLICY "Users can access active org tasks"
ON tasks FOR ALL
USING (
    EXISTS (
        SELECT 1 FROM jobs j
        WHERE j.id = tasks.job_id
          AND j.organisation_id = public.user_organisation_id()
          AND public.user_is_member_of(j.organisation_id)
    )
);

-- =============================================================================
-- SCHEDULERS TABLE
-- =============================================================================

DROP POLICY IF EXISTS "Users can view own org schedulers" ON schedulers;
DROP POLICY IF EXISTS "Users can create own org schedulers" ON schedulers;
DROP POLICY IF EXISTS "Users can update own org schedulers" ON schedulers;
DROP POLICY IF EXISTS "Users can delete own org schedulers" ON schedulers;

CREATE POLICY "Users can view active org schedulers"
ON schedulers FOR SELECT
USING (
    organisation_id = public.user_organisation_id()
    AND public.user_is_member_of(organisation_id)
);

CREATE POLICY "Users can create active org schedulers"
ON schedulers FOR INSERT
WITH CHECK (
    organisation_id = public.user_organisation_id()
    AND public.user_is_member_of(organisation_id)
);

CREATE POLICY "Users can update active org schedulers"
ON schedulers FOR UPDATE
USING (
    organisation_id = public.user_organisation_id()
    AND public.user_is_member_of(organisation_id)
)
WITH CHECK (
    organisation_id = public.user_organisation_id()
    AND public.user_is_member_of(organisation_id)
);

CREATE POLICY "Users can delete active org schedulers"
ON schedulers FOR DELETE
USING (
    organisation_id = public.user_organisation_id()
    AND public.user_is_member_of(organisation_id)
);

-- =============================================================================
-- JOB_SHARE_LINKS TABLE
-- =============================================================================

DROP POLICY IF EXISTS "Users can view own org share links" ON job_share_links;
DROP POLICY IF EXISTS "Users can create share links for own org jobs" ON job_share_links;
DROP POLICY IF EXISTS "Users can update own org share links" ON job_share_links;
DROP POLICY IF EXISTS "Users can delete own org share links" ON job_share_links;

CREATE POLICY "Users can view active org share links"
ON job_share_links FOR SELECT
USING (
    EXISTS (
        SELECT 1 FROM jobs j
        WHERE j.id = job_share_links.job_id
          AND j.organisation_id = public.user_organisation_id()
          AND public.user_is_member_of(j.organisation_id)
    )
);

CREATE POLICY "Users can create active org share links"
ON job_share_links FOR INSERT
WITH CHECK (
    EXISTS (
        SELECT 1 FROM jobs j
        WHERE j.id = job_share_links.job_id
          AND j.organisation_id = public.user_organisation_id()
          AND public.user_is_member_of(j.organisation_id)
    )
);

CREATE POLICY "Users can update active org share links"
ON job_share_links FOR UPDATE
USING (
    EXISTS (
        SELECT 1 FROM jobs j
        WHERE j.id = job_share_links.job_id
          AND j.organisation_id = public.user_organisation_id()
          AND public.user_is_member_of(j.organisation_id)
    )
)
WITH CHECK (
    EXISTS (
        SELECT 1 FROM jobs j
        WHERE j.id = job_share_links.job_id
          AND j.organisation_id = public.user_organisation_id()
          AND public.user_is_member_of(j.organisation_id)
    )
);

CREATE POLICY "Users can delete active org share links"
ON job_share_links FOR DELETE
USING (
    EXISTS (
        SELECT 1 FROM jobs j
        WHERE j.id = job_share_links.job_id
          AND j.organisation_id = public.user_organisation_id()
          AND public.user_is_member_of(j.organisation_id)
    )
);

-- =============================================================================
-- NOTES
-- =============================================================================

-- Domains and Pages RLS policies already work via jobs join - no changes needed
-- They inherit the new job policies automatically

-- The user_organisation_id() function uses COALESCE to fall back to legacy
-- organisation_id during transition, ensuring backward compatibility
