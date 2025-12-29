-- Link BBB users to Slack users for DM notifications
CREATE TABLE slack_user_links (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  slack_connection_id UUID NOT NULL REFERENCES slack_connections(id) ON DELETE CASCADE,
  slack_user_id TEXT NOT NULL,             -- Slack member ID
  dm_notifications BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(user_id, slack_connection_id)
);

CREATE INDEX idx_slack_user_links_connection ON slack_user_links(slack_connection_id);
CREATE INDEX idx_slack_user_links_user ON slack_user_links(user_id);

-- Enable RLS
ALTER TABLE slack_user_links ENABLE ROW LEVEL SECURITY;

-- RLS policies - users can manage their own links
CREATE POLICY "slack_user_links_select_own" ON slack_user_links
  FOR SELECT USING (user_id = auth.uid());

CREATE POLICY "slack_user_links_insert_own" ON slack_user_links
  FOR INSERT WITH CHECK (user_id = auth.uid());

CREATE POLICY "slack_user_links_update_own" ON slack_user_links
  FOR UPDATE USING (user_id = auth.uid());

CREATE POLICY "slack_user_links_delete_own" ON slack_user_links
  FOR DELETE USING (user_id = auth.uid());
