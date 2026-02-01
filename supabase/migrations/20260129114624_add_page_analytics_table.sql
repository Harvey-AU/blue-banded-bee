-- Org-scoped Page Analytics Table
-- Stores GA4 page view data tied to the organisation that fetched it
-- Prevents data leaking across organisations since domains aren't org-scoped

-- ============================================================================
-- 1. Create page_analytics table
-- ============================================================================
CREATE TABLE page_analytics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
    domain_id INTEGER NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    path TEXT NOT NULL,

    -- Analytics data (page views by time period)
    page_views_7d BIGINT DEFAULT 0,
    page_views_28d BIGINT DEFAULT 0,
    page_views_180d BIGINT DEFAULT 0,

    -- Source tracking
    ga_connection_id UUID REFERENCES google_analytics_connections(id) ON DELETE SET NULL,

    -- Timestamps
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One analytics record per org/domain/path combination
    UNIQUE(organisation_id, domain_id, path)
);

-- ============================================================================
-- 2. Create indexes
-- ============================================================================
CREATE INDEX idx_page_analytics_org ON page_analytics(organisation_id);
CREATE INDEX idx_page_analytics_domain ON page_analytics(domain_id);
CREATE INDEX idx_page_analytics_org_domain ON page_analytics(organisation_id, domain_id);
CREATE INDEX idx_page_analytics_fetched ON page_analytics(fetched_at);

-- ============================================================================
-- 3. Enable RLS and create policies
-- ============================================================================
ALTER TABLE page_analytics ENABLE ROW LEVEL SECURITY;

CREATE POLICY "page_analytics_select_own_org" ON page_analytics
    FOR SELECT USING (organisation_id IN (SELECT public.user_organisations()));

CREATE POLICY "page_analytics_insert_own_org" ON page_analytics
    FOR INSERT WITH CHECK (organisation_id IN (SELECT public.user_organisations()));

CREATE POLICY "page_analytics_update_own_org" ON page_analytics
    FOR UPDATE USING (organisation_id IN (SELECT public.user_organisations()))
    WITH CHECK (organisation_id IN (SELECT public.user_organisations()));

CREATE POLICY "page_analytics_delete_own_org" ON page_analytics
    FOR DELETE USING (organisation_id IN (SELECT public.user_organisations()));

-- ============================================================================
-- 4. Create updated_at trigger
-- ============================================================================
CREATE TRIGGER update_page_analytics_updated_at
    BEFORE UPDATE ON page_analytics
    FOR EACH ROW
    EXECUTE FUNCTION public.update_updated_at_column();
