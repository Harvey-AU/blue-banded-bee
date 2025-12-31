-- Slack Integration with Supabase Vault
-- Single consolidated migration for proper ordering

-- ============================================================================
-- 1. Enable Vault extension
-- ============================================================================
CREATE EXTENSION IF NOT EXISTS supabase_vault WITH SCHEMA vault;

-- ============================================================================
-- 2. Create utility function for updated_at triggers
-- ============================================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- 3. Add slack_user_id to users table
-- ============================================================================
ALTER TABLE users ADD COLUMN IF NOT EXISTS slack_user_id TEXT;
CREATE INDEX IF NOT EXISTS idx_users_slack_user_id ON users(slack_user_id) WHERE slack_user_id IS NOT NULL;

-- Sync slack_user_id from Supabase Auth metadata when user signs in with Slack OIDC
CREATE OR REPLACE FUNCTION sync_slack_user_id()
RETURNS TRIGGER AS $$
BEGIN
  -- Only sync if this is a Slack OIDC login
  IF NEW.raw_app_meta_data->>'provider' = 'slack_oidc' THEN
    UPDATE users
    SET slack_user_id = NEW.raw_user_meta_data->>'sub'
    WHERE id = NEW.id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Note: This trigger requires manual creation on auth.users which is managed by Supabase
-- CREATE TRIGGER on_auth_user_updated
--   AFTER UPDATE ON auth.users
--   FOR EACH ROW
--   EXECUTE FUNCTION sync_slack_user_id();

-- ============================================================================
-- 4. Create slack_connections table (with Vault integration from start)
-- ============================================================================
CREATE TABLE slack_connections (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
  workspace_id TEXT NOT NULL,              -- Slack team_id
  workspace_name TEXT,                     -- Slack team name for display
  vault_secret_name TEXT,                  -- Name of secret in Supabase Vault
  bot_user_id TEXT,                        -- Slack bot user ID
  installing_user_id UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(organisation_id, workspace_id)
);

CREATE INDEX idx_slack_connections_org ON slack_connections(organisation_id);

-- Enable RLS
ALTER TABLE slack_connections ENABLE ROW LEVEL SECURITY;

-- RLS policies
CREATE POLICY "slack_connections_select_own_org" ON slack_connections
  FOR SELECT USING (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

CREATE POLICY "slack_connections_insert_own_org" ON slack_connections
  FOR INSERT WITH CHECK (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

CREATE POLICY "slack_connections_update_own_org" ON slack_connections
  FOR UPDATE USING (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

CREATE POLICY "slack_connections_delete_own_org" ON slack_connections
  FOR DELETE USING (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

-- Updated at trigger
CREATE TRIGGER update_slack_connections_updated_at
  BEFORE UPDATE ON slack_connections
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- 5. Create slack_user_links table
-- ============================================================================
CREATE TABLE slack_user_links (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  slack_connection_id UUID NOT NULL REFERENCES slack_connections(id) ON DELETE CASCADE,
  slack_user_id TEXT NOT NULL,             -- User's Slack member ID
  dm_notifications BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(user_id, slack_connection_id)
);

CREATE INDEX idx_slack_user_links_connection ON slack_user_links(slack_connection_id);
CREATE INDEX idx_slack_user_links_user ON slack_user_links(user_id);

-- Enable RLS
ALTER TABLE slack_user_links ENABLE ROW LEVEL SECURITY;

-- RLS policies
CREATE POLICY "slack_user_links_select_own" ON slack_user_links
  FOR SELECT USING (user_id = auth.uid());

CREATE POLICY "slack_user_links_insert_own" ON slack_user_links
  FOR INSERT WITH CHECK (user_id = auth.uid());

CREATE POLICY "slack_user_links_update_own" ON slack_user_links
  FOR UPDATE USING (user_id = auth.uid());

CREATE POLICY "slack_user_links_delete_own" ON slack_user_links
  FOR DELETE USING (user_id = auth.uid());

-- ============================================================================
-- 6. Create notifications table
-- ============================================================================
CREATE TABLE notifications (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
  user_id UUID REFERENCES users(id) ON DELETE SET NULL,  -- Optional: specific user
  type TEXT NOT NULL,                      -- 'job_complete', 'job_failed', etc.
  subject TEXT NOT NULL,                   -- Main heading (e.g., "example.com completed")
  preview TEXT,                            -- Short summary for toasts/previews
  message TEXT,                            -- Full details (optional)
  link TEXT,                               -- URL path (e.g., "/jobs/abc-123")
  data JSONB,                              -- Additional structured data
  read_at TIMESTAMPTZ,                     -- When user read in-app
  slack_delivered_at TIMESTAMPTZ,          -- When sent via Slack
  email_delivered_at TIMESTAMPTZ,          -- When sent via email (future)
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_org ON notifications(organisation_id);
CREATE INDEX idx_notifications_user ON notifications(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_notifications_unread ON notifications(organisation_id, read_at) WHERE read_at IS NULL;
CREATE INDEX idx_notifications_pending_slack ON notifications(created_at)
  WHERE slack_delivered_at IS NULL;

-- Enable RLS
ALTER TABLE notifications ENABLE ROW LEVEL SECURITY;

-- RLS policies
CREATE POLICY "notifications_select_own_org" ON notifications
  FOR SELECT USING (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

CREATE POLICY "notifications_update_own" ON notifications
  FOR UPDATE USING (
    user_id = auth.uid()
  );

-- ============================================================================
-- 7. Vault helper functions for Slack tokens
-- ============================================================================

-- Store a Slack token in Vault
CREATE OR REPLACE FUNCTION store_slack_token(connection_id UUID, token TEXT)
RETURNS TEXT AS $$
DECLARE
  secret_name TEXT;
  existing_secret_id UUID;
BEGIN
  secret_name := 'slack_token_' || connection_id::TEXT;

  -- Check if secret already exists
  SELECT id INTO existing_secret_id FROM vault.secrets WHERE name = secret_name;

  IF existing_secret_id IS NOT NULL THEN
    -- Update existing secret atomically
    UPDATE vault.secrets SET secret = token WHERE id = existing_secret_id;
  ELSE
    -- Create new secret
    PERFORM vault.create_secret(token, secret_name);
  END IF;

  -- Update connection with secret name
  UPDATE slack_connections
  SET vault_secret_name = secret_name
  WHERE id = connection_id;

  RETURN secret_name;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Retrieve decrypted token from Vault
CREATE OR REPLACE FUNCTION get_slack_token(connection_id UUID)
RETURNS TEXT AS $$
DECLARE
  token TEXT;
  secret_name TEXT;
BEGIN
  secret_name := 'slack_token_' || connection_id::TEXT;

  SELECT decrypted_secret INTO token
  FROM vault.decrypted_secrets
  WHERE name = secret_name;

  RETURN token;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Delete token when disconnecting
CREATE OR REPLACE FUNCTION delete_slack_token(connection_id UUID)
RETURNS VOID AS $$
DECLARE
  secret_name TEXT;
BEGIN
  secret_name := 'slack_token_' || connection_id::TEXT;
  DELETE FROM vault.secrets WHERE name = secret_name;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Auto-delete vault secret when connection is deleted
CREATE OR REPLACE FUNCTION cleanup_slack_vault_secret()
RETURNS TRIGGER AS $$
BEGIN
  PERFORM delete_slack_token(OLD.id);
  RETURN OLD;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE TRIGGER on_slack_connection_delete
  BEFORE DELETE ON slack_connections
  FOR EACH ROW
  EXECUTE FUNCTION cleanup_slack_vault_secret();

-- ============================================================================
-- 8. Auto-linking triggers
-- ============================================================================

-- Auto-link users to Slack when they sign in with Slack OIDC
CREATE OR REPLACE FUNCTION auto_link_slack_user()
RETURNS TRIGGER AS $$
BEGIN
  IF NEW.slack_user_id IS NOT NULL AND NEW.organisation_id IS NOT NULL THEN
    INSERT INTO slack_user_links (id, user_id, slack_connection_id, slack_user_id, dm_notifications, created_at)
    SELECT
      gen_random_uuid(),
      NEW.id,
      sc.id,
      NEW.slack_user_id,
      true,
      NOW()
    FROM slack_connections sc
    WHERE sc.organisation_id = NEW.organisation_id
    ON CONFLICT (user_id, slack_connection_id) DO UPDATE
      SET slack_user_id = EXCLUDED.slack_user_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE TRIGGER on_user_slack_link
  AFTER INSERT OR UPDATE OF slack_user_id ON users
  FOR EACH ROW
  WHEN (NEW.slack_user_id IS NOT NULL)
  EXECUTE FUNCTION auto_link_slack_user();

-- Auto-link existing Slack users when new connection is created
CREATE OR REPLACE FUNCTION auto_link_existing_slack_users()
RETURNS TRIGGER AS $$
BEGIN
  INSERT INTO slack_user_links (id, user_id, slack_connection_id, slack_user_id, dm_notifications, created_at)
  SELECT
    gen_random_uuid(),
    u.id,
    NEW.id,
    u.slack_user_id,
    true,
    NOW()
  FROM users u
  WHERE u.organisation_id = NEW.organisation_id
    AND u.slack_user_id IS NOT NULL
  ON CONFLICT (user_id, slack_connection_id) DO NOTHING;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE TRIGGER on_slack_connection_create
  AFTER INSERT ON slack_connections
  FOR EACH ROW
  EXECUTE FUNCTION auto_link_existing_slack_users();
