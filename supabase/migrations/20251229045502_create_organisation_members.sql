-- Migration: Create organisation_members table for many-to-many user-org relationships
-- Part of platform auth implementation for multi-organisation support

-- =============================================================================
-- TABLE CREATION
-- =============================================================================

CREATE TABLE IF NOT EXISTS organisation_members (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, organisation_id)
);

COMMENT ON TABLE organisation_members IS 'Many-to-many relationship between users and organisations';

-- =============================================================================
-- INDEXES
-- =============================================================================

-- Reverse lookup: find all members of an organisation
CREATE INDEX IF NOT EXISTS idx_org_members_org
ON organisation_members(organisation_id);

-- =============================================================================
-- ROW LEVEL SECURITY
-- =============================================================================

ALTER TABLE organisation_members ENABLE ROW LEVEL SECURITY;

-- Users can view their own memberships
CREATE POLICY "Users can view own memberships"
ON organisation_members FOR SELECT
USING (user_id = (SELECT auth.uid()));

-- Users can view other members of organisations they belong to
CREATE POLICY "Users can view org co-members"
ON organisation_members FOR SELECT
USING (
    organisation_id IN (
        SELECT om.organisation_id
        FROM organisation_members om
        WHERE om.user_id = (SELECT auth.uid())
    )
);

-- Service role can manage all memberships (for backend operations)
-- Note: INSERT/UPDATE/DELETE operations will be handled by the backend using service role
