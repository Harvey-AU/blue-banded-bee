-- Migration: Add active_organisation_id for organisation context switching
-- Part of platform auth implementation for multi-organisation support

-- =============================================================================
-- SCHEMA CHANGES
-- =============================================================================

-- Add column to track user's currently active organisation
ALTER TABLE users
ADD COLUMN IF NOT EXISTS active_organisation_id UUID REFERENCES organisations(id);

COMMENT ON COLUMN users.active_organisation_id IS 'Currently selected organisation for multi-org users. Falls back to organisation_id during transition.';

-- Backfill with current organisation_id (preserves existing behaviour)
UPDATE users
SET active_organisation_id = organisation_id
WHERE organisation_id IS NOT NULL
  AND active_organisation_id IS NULL;

-- =============================================================================
-- INDEXES
-- =============================================================================

-- Index for efficient lookup during RLS policy evaluation
CREATE INDEX IF NOT EXISTS idx_users_active_org
ON users(active_organisation_id)
WHERE active_organisation_id IS NOT NULL;

-- =============================================================================
-- HELPER FUNCTIONS
-- =============================================================================

-- Update existing helper function with backward compatibility
-- Uses COALESCE to fall back to legacy organisation_id during transition
CREATE OR REPLACE FUNCTION public.user_organisation_id()
RETURNS uuid AS $$
    SELECT COALESCE(active_organisation_id, organisation_id)
    FROM users
    WHERE id = auth.uid()
$$ LANGUAGE sql STABLE SECURITY DEFINER;

COMMENT ON FUNCTION public.user_organisation_id() IS 'Returns active org, falling back to legacy organisation_id for backward compatibility';

-- New function to check if user is a member of a specific organisation
CREATE OR REPLACE FUNCTION public.user_is_member_of(org_id uuid)
RETURNS boolean AS $$
    SELECT EXISTS (
        SELECT 1 FROM organisation_members
        WHERE user_id = auth.uid()
          AND organisation_id = org_id
    )
$$ LANGUAGE sql STABLE SECURITY DEFINER;

COMMENT ON FUNCTION public.user_is_member_of(uuid) IS 'Checks if current user is a member of the specified organisation';

-- New function to get all organisations the user belongs to
CREATE OR REPLACE FUNCTION public.user_organisations()
RETURNS SETOF uuid AS $$
    SELECT organisation_id
    FROM organisation_members
    WHERE user_id = auth.uid()
$$ LANGUAGE sql STABLE SECURITY DEFINER;

COMMENT ON FUNCTION public.user_organisations() IS 'Returns all organisation IDs the current user is a member of';
