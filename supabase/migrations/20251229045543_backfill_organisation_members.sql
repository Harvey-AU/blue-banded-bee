-- Migration: Backfill organisation_members from existing users.organisation_id
-- Part of platform auth implementation for multi-organisation support

-- =============================================================================
-- BACKFILL DATA
-- =============================================================================

-- Insert existing user-org relationships from the legacy organisation_id column
-- Existing users become members of their current organisation
INSERT INTO organisation_members (user_id, organisation_id, created_at)
SELECT
    u.id AS user_id,
    u.organisation_id,
    u.created_at  -- Preserve original relationship timestamp
FROM users u
WHERE u.organisation_id IS NOT NULL
ON CONFLICT (user_id, organisation_id) DO NOTHING;  -- Idempotent for safe re-runs

-- =============================================================================
-- VERIFICATION
-- =============================================================================

DO $$
DECLARE
    users_with_org INTEGER;
    members_created INTEGER;
BEGIN
    SELECT COUNT(*) INTO users_with_org FROM users WHERE organisation_id IS NOT NULL;
    SELECT COUNT(*) INTO members_created FROM organisation_members;

    RAISE NOTICE 'Backfill complete: % users with organisations, % memberships created',
                 users_with_org, members_created;
END $$;
