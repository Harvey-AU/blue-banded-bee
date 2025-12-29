-- Switch from custom encryption to Supabase Vault for Slack tokens
-- This is a fresh start - no existing tokens to migrate

-- Enable vault extension if not already enabled
CREATE EXTENSION IF NOT EXISTS supabase_vault WITH SCHEMA vault;

-- Remove custom encryption columns
ALTER TABLE slack_connections
  DROP COLUMN IF EXISTS access_token_encrypted,
  DROP COLUMN IF EXISTS access_token_nonce;

-- Add vault secret name column (stores the name used in vault.secrets)
-- Secret name format: 'slack_token_{connection_id}'
ALTER TABLE slack_connections
  ADD COLUMN vault_secret_name TEXT;

-- Helper function to store a Slack token in Vault
-- Called from Go after OAuth token exchange
CREATE OR REPLACE FUNCTION store_slack_token(connection_id UUID, token TEXT)
RETURNS TEXT AS $$
DECLARE
  secret_name TEXT;
BEGIN
  secret_name := 'slack_token_' || connection_id::TEXT;

  -- Delete existing secret if any
  DELETE FROM vault.secrets WHERE name = secret_name;

  -- Create new secret
  PERFORM vault.create_secret(token, secret_name);

  -- Update connection with secret name
  UPDATE slack_connections
  SET vault_secret_name = secret_name
  WHERE id = connection_id;

  RETURN secret_name;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Helper function to retrieve decrypted token
-- Called from Go when sending Slack messages
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

-- Helper function to delete token when disconnecting
CREATE OR REPLACE FUNCTION delete_slack_token(connection_id UUID)
RETURNS VOID AS $$
DECLARE
  secret_name TEXT;
BEGIN
  secret_name := 'slack_token_' || connection_id::TEXT;
  DELETE FROM vault.secrets WHERE name = secret_name;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Trigger to auto-delete vault secret when connection is deleted
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
