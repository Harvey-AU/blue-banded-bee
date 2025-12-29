-- Slack workspace connections for organisations
CREATE TABLE slack_connections (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
  workspace_id TEXT NOT NULL,              -- Slack team_id
  workspace_name TEXT,                     -- Slack team name for display
  access_token_encrypted TEXT NOT NULL,    -- AES-256-GCM encrypted token
  access_token_nonce TEXT NOT NULL,        -- Nonce for decryption
  bot_user_id TEXT,                        -- Slack bot user ID
  installing_user_id UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(organisation_id, workspace_id)
);

CREATE INDEX idx_slack_connections_org ON slack_connections(organisation_id);

-- Enable RLS
ALTER TABLE slack_connections ENABLE ROW LEVEL SECURITY;

-- RLS policies (following scheduler pattern)
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
