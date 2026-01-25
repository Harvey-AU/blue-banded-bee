-- Google Analytics Integration with Supabase Vault
-- Following Webflow integration pattern

-- ============================================================================
-- 1. Create google_analytics_connections table
-- ============================================================================
CREATE TABLE google_analytics_connections (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
  ga4_property_id TEXT,                   -- GA4 property ID (e.g., "123456789")
  ga4_property_name TEXT,                 -- Display name of the GA4 property
  google_user_id TEXT,                    -- Google user ID who authorised
  google_email TEXT,                      -- Google email for display
  vault_secret_name TEXT,                 -- Name of secret in Supabase Vault (stores refresh token)
  installing_user_id UUID REFERENCES users(id),
  last_synced_at TIMESTAMPTZ,             -- When analytics data was last synced
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  -- One connection per org+property combination
  UNIQUE(organisation_id, ga4_property_id)
);

CREATE INDEX idx_ga_connections_org ON google_analytics_connections(organisation_id);
CREATE INDEX idx_ga_connections_property ON google_analytics_connections(ga4_property_id);

-- Enable RLS
ALTER TABLE google_analytics_connections ENABLE ROW LEVEL SECURITY;

-- RLS policies
CREATE POLICY "ga_connections_select_own_org" ON google_analytics_connections
  FOR SELECT USING (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

CREATE POLICY "ga_connections_insert_own_org" ON google_analytics_connections
  FOR INSERT WITH CHECK (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

CREATE POLICY "ga_connections_update_own_org" ON google_analytics_connections
  FOR UPDATE USING (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

CREATE POLICY "ga_connections_delete_own_org" ON google_analytics_connections
  FOR DELETE USING (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

-- Updated at trigger
CREATE TRIGGER update_ga_connections_updated_at
  BEFORE UPDATE ON google_analytics_connections
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at_column();


-- ============================================================================
-- 2. Vault helper functions for Google Analytics tokens
-- ============================================================================

-- Store a Google Analytics refresh token in Vault
-- We store the refresh token since access tokens expire in 1 hour
CREATE OR REPLACE FUNCTION store_ga_token(connection_id UUID, refresh_token TEXT)
RETURNS TEXT AS $$
DECLARE
  secret_name TEXT;
BEGIN
  secret_name := 'ga_token_' || connection_id::TEXT;

  -- Use atomic upsert pattern - delete existing and create new
  DELETE FROM vault.secrets WHERE name = secret_name;
  PERFORM vault.create_secret(refresh_token, secret_name);

  -- Update connection with secret name
  UPDATE google_analytics_connections
  SET vault_secret_name = secret_name
  WHERE id = connection_id;

  RETURN secret_name;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Retrieve decrypted refresh token from Vault
CREATE OR REPLACE FUNCTION get_ga_token(connection_id UUID)
RETURNS TEXT AS $$
DECLARE
  token TEXT;
  secret_name TEXT;
BEGIN
  secret_name := 'ga_token_' || connection_id::TEXT;

  SELECT decrypted_secret INTO token
  FROM vault.decrypted_secrets
  WHERE name = secret_name;

  RETURN token;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Delete token when disconnecting
CREATE OR REPLACE FUNCTION delete_ga_token(connection_id UUID)
RETURNS VOID AS $$
DECLARE
  secret_name TEXT;
BEGIN
  secret_name := 'ga_token_' || connection_id::TEXT;
  DELETE FROM vault.secrets WHERE name = secret_name;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Auto-delete vault secret when connection is deleted
CREATE OR REPLACE FUNCTION cleanup_ga_vault_secret()
RETURNS TRIGGER AS $$
BEGIN
  PERFORM delete_ga_token(OLD.id);
  RETURN OLD;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE TRIGGER on_ga_connection_delete
  BEFORE DELETE ON google_analytics_connections
  FOR EACH ROW
  EXECUTE FUNCTION cleanup_ga_vault_secret();
