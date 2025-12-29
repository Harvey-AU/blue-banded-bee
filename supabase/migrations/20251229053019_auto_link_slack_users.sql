-- Auto-link users to Slack connections when they sign in with Slack
-- This creates entries in slack_user_links for all of the org's connections

CREATE OR REPLACE FUNCTION auto_link_slack_user()
RETURNS TRIGGER AS $$
BEGIN
  -- Only process if slack_user_id was just set (not null) and org is set
  IF NEW.slack_user_id IS NOT NULL AND NEW.organisation_id IS NOT NULL THEN
    -- Auto-create links for all of the user's org's Slack connections
    INSERT INTO slack_user_links (id, user_id, slack_connection_id, slack_user_id, dm_notifications, created_at)
    SELECT
      gen_random_uuid(),
      NEW.id,
      sc.id,
      NEW.slack_user_id,
      true,  -- Enable DM notifications by default
      NOW()
    FROM slack_connections sc
    WHERE sc.organisation_id = NEW.organisation_id
    ON CONFLICT (user_id, slack_connection_id) DO UPDATE
      SET slack_user_id = EXCLUDED.slack_user_id;  -- Update slack_user_id if it changed
  END IF;

  RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Trigger on users table when slack_user_id changes
CREATE TRIGGER on_user_slack_link
  AFTER INSERT OR UPDATE OF slack_user_id ON users
  FOR EACH ROW
  WHEN (NEW.slack_user_id IS NOT NULL)
  EXECUTE FUNCTION auto_link_slack_user();

-- Also auto-link existing Slack users when a new connection is created
CREATE OR REPLACE FUNCTION auto_link_existing_slack_users()
RETURNS TRIGGER AS $$
BEGIN
  -- Link all users in this org who have a slack_user_id
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
