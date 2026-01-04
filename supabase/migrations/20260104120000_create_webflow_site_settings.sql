-- Create webflow_site_settings table for per-site configuration
-- Each site can have its own schedule and auto-publish webhook settings

CREATE TABLE IF NOT EXISTS webflow_site_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Link to connection and organisation
    connection_id UUID NOT NULL REFERENCES webflow_connections(id) ON DELETE CASCADE,
    organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,

    -- Webflow site identification
    webflow_site_id TEXT NOT NULL,
    site_name TEXT,
    primary_domain TEXT,

    -- Per-site settings
    schedule_interval_hours INTEGER CHECK (schedule_interval_hours IS NULL OR schedule_interval_hours IN (6, 12, 24, 48)),
    auto_publish_enabled BOOLEAN NOT NULL DEFAULT FALSE,

    -- Webhook tracking (for deletion when toggled off)
    webhook_id TEXT,
    webhook_registered_at TIMESTAMPTZ,

    -- Scheduler link (NULL if no schedule configured)
    scheduler_id UUID REFERENCES schedulers(id) ON DELETE SET NULL,

    -- Metadata
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One setting per site per organisation
    CONSTRAINT unique_site_per_org UNIQUE(organisation_id, webflow_site_id)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_webflow_site_settings_connection
    ON webflow_site_settings(connection_id);
CREATE INDEX IF NOT EXISTS idx_webflow_site_settings_org
    ON webflow_site_settings(organisation_id);
CREATE INDEX IF NOT EXISTS idx_webflow_site_settings_site_id
    ON webflow_site_settings(webflow_site_id);
CREATE INDEX IF NOT EXISTS idx_webflow_site_settings_scheduler
    ON webflow_site_settings(scheduler_id) WHERE scheduler_id IS NOT NULL;

-- Enable RLS
ALTER TABLE webflow_site_settings ENABLE ROW LEVEL SECURITY;

-- RLS Policies: Users can only access their organisation's site settings
CREATE POLICY "webflow_site_settings_select_own_org" ON webflow_site_settings
    FOR SELECT USING (
        organisation_id IN (SELECT public.user_organisations())
    );

CREATE POLICY "webflow_site_settings_insert_own_org" ON webflow_site_settings
    FOR INSERT WITH CHECK (
        organisation_id IN (SELECT public.user_organisations())
    );

CREATE POLICY "webflow_site_settings_update_own_org" ON webflow_site_settings
    FOR UPDATE USING (
        organisation_id IN (SELECT public.user_organisations())
    );

CREATE POLICY "webflow_site_settings_delete_own_org" ON webflow_site_settings
    FOR DELETE USING (
        organisation_id IN (SELECT public.user_organisations())
    );

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_webflow_site_settings_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER webflow_site_settings_updated_at_trigger
    BEFORE UPDATE ON webflow_site_settings
    FOR EACH ROW
    EXECUTE FUNCTION update_webflow_site_settings_updated_at();

-- Comment on table
COMMENT ON TABLE webflow_site_settings IS 'Per-Webflow-site configuration for schedules and auto-publish webhooks';
