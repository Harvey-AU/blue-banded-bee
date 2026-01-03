-- Update integration RLS policies and Slack auto-linking for multi-organisation support

-- ============================================================================
-- Slack connections RLS (use organisation membership)
-- ============================================================================
ALTER TABLE slack_connections ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS "slack_connections_select_own_org" ON slack_connections;
DROP POLICY IF EXISTS "slack_connections_insert_own_org" ON slack_connections;
DROP POLICY IF EXISTS "slack_connections_update_own_org" ON slack_connections;
DROP POLICY IF EXISTS "slack_connections_delete_own_org" ON slack_connections;

CREATE POLICY "slack_connections_select_own_org" ON slack_connections
  FOR SELECT USING (
    organisation_id IN (SELECT public.user_organisations())
  );

CREATE POLICY "slack_connections_insert_own_org" ON slack_connections
  FOR INSERT WITH CHECK (
    organisation_id IN (SELECT public.user_organisations())
  );

CREATE POLICY "slack_connections_update_own_org" ON slack_connections
  FOR UPDATE USING (
    organisation_id IN (SELECT public.user_organisations())
  );

CREATE POLICY "slack_connections_delete_own_org" ON slack_connections
  FOR DELETE USING (
    organisation_id IN (SELECT public.user_organisations())
  );

-- ============================================================================
-- Webflow connections RLS (use organisation membership)
-- ============================================================================
ALTER TABLE webflow_connections ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS "webflow_connections_select_own_org" ON webflow_connections;
DROP POLICY IF EXISTS "webflow_connections_insert_own_org" ON webflow_connections;
DROP POLICY IF EXISTS "webflow_connections_update_own_org" ON webflow_connections;
DROP POLICY IF EXISTS "webflow_connections_delete_own_org" ON webflow_connections;

CREATE POLICY "webflow_connections_select_own_org" ON webflow_connections
  FOR SELECT USING (
    organisation_id IN (SELECT public.user_organisations())
  );

CREATE POLICY "webflow_connections_insert_own_org" ON webflow_connections
  FOR INSERT WITH CHECK (
    organisation_id IN (SELECT public.user_organisations())
  );

CREATE POLICY "webflow_connections_update_own_org" ON webflow_connections
  FOR UPDATE USING (
    organisation_id IN (SELECT public.user_organisations())
  );

CREATE POLICY "webflow_connections_delete_own_org" ON webflow_connections
  FOR DELETE USING (
    organisation_id IN (SELECT public.user_organisations())
  );

-- ============================================================================
-- Slack auto-linking (use active organisation)
-- ============================================================================
CREATE OR REPLACE FUNCTION auto_link_slack_user()
RETURNS TRIGGER AS $$
BEGIN
  IF NEW.slack_user_id IS NOT NULL AND NEW.active_organisation_id IS NOT NULL THEN
    INSERT INTO slack_user_links (id, user_id, slack_connection_id, slack_user_id, dm_notifications, created_at)
    SELECT
      gen_random_uuid(),
      NEW.id,
      sc.id,
      NEW.slack_user_id,
      true,
      NOW()
    FROM slack_connections sc
    WHERE sc.organisation_id = NEW.active_organisation_id
    ON CONFLICT (user_id, slack_connection_id) DO UPDATE
      SET slack_user_id = EXCLUDED.slack_user_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

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
  WHERE u.active_organisation_id = NEW.organisation_id
    AND u.slack_user_id IS NOT NULL
  ON CONFLICT (user_id, slack_connection_id) DO NOTHING;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
