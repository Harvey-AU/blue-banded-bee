-- Add slack_user_id column to users table
-- This is populated automatically when a user signs in with Slack OIDC

ALTER TABLE users ADD COLUMN slack_user_id TEXT;

CREATE INDEX idx_users_slack_user_id ON users(slack_user_id) WHERE slack_user_id IS NOT NULL;

-- Trigger to sync slack_user_id from Supabase Auth when user signs in with Slack
CREATE OR REPLACE FUNCTION sync_slack_user_id()
RETURNS TRIGGER AS $$
BEGIN
  -- Only process if this is a Slack OIDC login
  IF NEW.raw_app_meta_data->>'provider' = 'slack_oidc' THEN
    UPDATE public.users
    SET slack_user_id = NEW.raw_user_meta_data->>'sub',
        updated_at = NOW()
    WHERE id = NEW.id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Trigger on auth.users updates (fires when user signs in)
CREATE TRIGGER on_auth_user_slack_sync
  AFTER INSERT OR UPDATE ON auth.users
  FOR EACH ROW
  EXECUTE FUNCTION sync_slack_user_id();
