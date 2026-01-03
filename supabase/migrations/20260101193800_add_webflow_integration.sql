-- Webflow Integration with Supabase Vault
-- Mirrored from Slack integration pattern

-- ============================================================================
-- 1. Create webflow_connections table
-- ============================================================================
CREATE TABLE webflow_connections (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
  webflow_workspace_id TEXT,               -- Webflow workspace ID (context only)
  authed_user_id TEXT,                     -- Webflow user ID (who authed)
  vault_secret_name TEXT,                  -- Name of secret in Supabase Vault
  installing_user_id UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  -- Unlike Slack, we might have multiple connections if we link different Sites from different Workspaces?
  -- But usually OAuth is User-scoped for Workspaces.
  -- Let's assume one connection per Org for now, similar to Slack, OR allow multiple.
  -- The plan says "mapping webflow + site_id -> organisation_id".
  -- This table stores the AUTH connection (Token).
  -- A Token spans a whole Workspace usually? No, Site Token spans a Site.
  -- We are using Data Client (App) which gives access to selected Sites.
  -- So this connection represents the "App Installation" or "User Authorization".
  UNIQUE(organisation_id, authed_user_id)
);

CREATE INDEX idx_webflow_connections_org ON webflow_connections(organisation_id);

-- Enable RLS
ALTER TABLE webflow_connections ENABLE ROW LEVEL SECURITY;

-- RLS policies
CREATE POLICY "webflow_connections_select_own_org" ON webflow_connections
  FOR SELECT USING (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

CREATE POLICY "webflow_connections_insert_own_org" ON webflow_connections
  FOR INSERT WITH CHECK (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

CREATE POLICY "webflow_connections_update_own_org" ON webflow_connections
  FOR UPDATE USING (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

CREATE POLICY "webflow_connections_delete_own_org" ON webflow_connections
  FOR DELETE USING (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

-- Updated at trigger
CREATE TRIGGER update_webflow_connections_updated_at
  BEFORE UPDATE ON webflow_connections
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at_column();


-- ============================================================================
-- 2. Vault helper functions for Webflow tokens
-- ============================================================================

-- Store a Webflow token in Vault
CREATE OR REPLACE FUNCTION store_webflow_token(connection_id UUID, token TEXT)
RETURNS TEXT AS $$
DECLARE
  secret_name TEXT;
BEGIN
  secret_name := 'webflow_token_' || connection_id::TEXT;

  -- Use atomic upsert pattern - delete existing and create new
  -- This avoids race condition between check and insert
  DELETE FROM vault.secrets WHERE name = secret_name;
  PERFORM vault.create_secret(token, secret_name);

  -- Update connection with secret name
  UPDATE webflow_connections
  SET vault_secret_name = secret_name
  WHERE id = connection_id;

  RETURN secret_name;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Retrieve decrypted token from Vault
CREATE OR REPLACE FUNCTION get_webflow_token(connection_id UUID)
RETURNS TEXT AS $$
DECLARE
  token TEXT;
  secret_name TEXT;
BEGIN
  secret_name := 'webflow_token_' || connection_id::TEXT;

  SELECT decrypted_secret INTO token
  FROM vault.decrypted_secrets
  WHERE name = secret_name;

  RETURN token;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Delete token when disconnecting
CREATE OR REPLACE FUNCTION delete_webflow_token(connection_id UUID)
RETURNS VOID AS $$
DECLARE
  secret_name TEXT;
BEGIN
  secret_name := 'webflow_token_' || connection_id::TEXT;
  DELETE FROM vault.secrets WHERE name = secret_name;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Auto-delete vault secret when connection is deleted
CREATE OR REPLACE FUNCTION cleanup_webflow_vault_secret()
RETURNS TRIGGER AS $$
BEGIN
  PERFORM delete_webflow_token(OLD.id);
  RETURN OLD;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE TRIGGER on_webflow_connection_delete
  BEFORE DELETE ON webflow_connections
  FOR EACH ROW
  EXECUTE FUNCTION cleanup_webflow_vault_secret();
