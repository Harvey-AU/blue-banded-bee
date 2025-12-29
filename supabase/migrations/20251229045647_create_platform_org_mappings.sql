-- Migration: Create platform_org_mappings for Webflow/Shopify workspace mapping
-- Part of platform auth implementation for multi-organisation support

-- =============================================================================
-- TABLE CREATION
-- =============================================================================

CREATE TABLE IF NOT EXISTS platform_org_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Platform identification
    platform TEXT NOT NULL CHECK (platform IN ('webflow', 'shopify')),
    platform_id TEXT NOT NULL,   -- Workspace ID (Webflow) or Shop/Org ID (Shopify)
    platform_name TEXT,          -- Human-readable name for UI

    -- BBB organisation link
    organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,

    -- Metadata
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID REFERENCES users(id),  -- Who created the mapping

    -- Each platform+platform_id combo maps to exactly one org
    CONSTRAINT unique_platform_mapping UNIQUE(platform, platform_id)
);

COMMENT ON TABLE platform_org_mappings IS 'Maps external platform identities (Webflow workspaces, Shopify stores) to BBB organisations';
COMMENT ON COLUMN platform_org_mappings.platform IS 'Platform type: webflow or shopify';
COMMENT ON COLUMN platform_org_mappings.platform_id IS 'Webflow workspace ID or Shopify shop/org ID';
COMMENT ON COLUMN platform_org_mappings.platform_name IS 'Human-readable platform name (e.g., workspace or store name)';

-- =============================================================================
-- INDEXES
-- =============================================================================

-- Primary lookup: resolve platform identity to org
CREATE INDEX IF NOT EXISTS idx_platform_mappings_lookup
ON platform_org_mappings(platform, platform_id);

-- Find all mappings for an organisation
CREATE INDEX IF NOT EXISTS idx_platform_mappings_org
ON platform_org_mappings(organisation_id);

-- =============================================================================
-- ROW LEVEL SECURITY
-- =============================================================================

ALTER TABLE platform_org_mappings ENABLE ROW LEVEL SECURITY;

-- Users can view mappings for organisations they belong to
CREATE POLICY "Users can view own org mappings"
ON platform_org_mappings FOR SELECT
USING (
    organisation_id IN (SELECT public.user_organisations())
);

-- Service role can manage all mappings (for backend operations)
-- Note: INSERT/UPDATE/DELETE operations will be handled by the backend using service role

-- =============================================================================
-- HELPER FUNCTION
-- =============================================================================

-- Resolve platform identity to BBB organisation
-- Uses SECURITY INVOKER so RLS applies - backend uses service role to bypass when needed
CREATE OR REPLACE FUNCTION public.resolve_platform_org(
    p_platform TEXT,
    p_platform_id TEXT
)
RETURNS uuid AS $$
    SELECT organisation_id
    FROM platform_org_mappings
    WHERE platform = p_platform
      AND platform_id = p_platform_id
$$ LANGUAGE sql STABLE SECURITY INVOKER;

COMMENT ON FUNCTION public.resolve_platform_org(TEXT, TEXT) IS 'Returns the BBB organisation_id for a given platform identity';
