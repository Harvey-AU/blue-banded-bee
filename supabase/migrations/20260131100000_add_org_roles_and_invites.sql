-- Migration: Add organisation roles and invites

-- =============================================================================
-- STEP 1: Add role to organisation_members
-- =============================================================================
ALTER TABLE organisation_members
ADD COLUMN IF NOT EXISTS role TEXT;

UPDATE organisation_members
SET role = 'member'
WHERE role IS NULL;

ALTER TABLE organisation_members
ALTER COLUMN role SET DEFAULT 'member';

ALTER TABLE organisation_members
ALTER COLUMN role SET NOT NULL;

ALTER TABLE organisation_members
ADD CONSTRAINT organisation_members_role_check
CHECK (role IN ('admin', 'member'));

-- Ensure each organisation has at least one admin (earliest member)
WITH ranked_members AS (
    SELECT organisation_id, user_id,
           ROW_NUMBER() OVER (PARTITION BY organisation_id ORDER BY created_at ASC) AS rn
    FROM organisation_members
)
UPDATE organisation_members om
SET role = 'admin'
FROM ranked_members rm
WHERE om.organisation_id = rm.organisation_id
  AND om.user_id = rm.user_id
  AND rm.rn = 1;

-- =============================================================================
-- STEP 2: Create organisation_invites table
-- =============================================================================
CREATE TABLE IF NOT EXISTS organisation_invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'member')),
    token TEXT NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);

COMMENT ON TABLE organisation_invites IS 'Pending organisation invitations sent via Supabase Auth.';

CREATE UNIQUE INDEX IF NOT EXISTS organisation_invites_token_idx
ON organisation_invites(token);

CREATE INDEX IF NOT EXISTS organisation_invites_org_idx
ON organisation_invites(organisation_id);

CREATE INDEX IF NOT EXISTS organisation_invites_email_idx
ON organisation_invites(lower(email));

-- Only allow one active invite per org/email
CREATE UNIQUE INDEX IF NOT EXISTS organisation_invites_unique_pending
ON organisation_invites(organisation_id, lower(email))
WHERE accepted_at IS NULL AND revoked_at IS NULL;

-- =============================================================================
-- STEP 3: Row-level security (admin only)
-- =============================================================================
ALTER TABLE organisation_invites ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS "Users can view org invites" ON organisation_invites;

DROP POLICY IF EXISTS "Admins can manage org invites" ON organisation_invites;

DROP POLICY IF EXISTS "Admins can insert org invites" ON organisation_invites;

DROP POLICY IF EXISTS "Admins can update org invites" ON organisation_invites;

DROP POLICY IF EXISTS "Admins can delete org invites" ON organisation_invites;

CREATE POLICY "Users can view org invites"
ON organisation_invites FOR SELECT
USING (
    organisation_id IN (
        SELECT om.organisation_id
        FROM organisation_members om
        WHERE om.user_id = (SELECT auth.uid())
          AND om.role = 'admin'
    )
);

CREATE POLICY "Admins can insert org invites"
ON organisation_invites
FOR INSERT
WITH CHECK (
    organisation_id IN (
        SELECT om.organisation_id
        FROM organisation_members om
        WHERE om.user_id = (SELECT auth.uid())
          AND om.role = 'admin'
    )
);

CREATE POLICY "Admins can update org invites"
ON organisation_invites
FOR UPDATE
USING (
    organisation_id IN (
        SELECT om.organisation_id
        FROM organisation_members om
        WHERE om.user_id = (SELECT auth.uid())
          AND om.role = 'admin'
    )
)
WITH CHECK (
    organisation_id IN (
        SELECT om.organisation_id
        FROM organisation_members om
        WHERE om.user_id = (SELECT auth.uid())
          AND om.role = 'admin'
    )
);

CREATE POLICY "Admins can delete org invites"
ON organisation_invites
FOR DELETE
USING (
    organisation_id IN (
        SELECT om.organisation_id
        FROM organisation_members om
        WHERE om.user_id = (SELECT auth.uid())
          AND om.role = 'admin'
    )
);
