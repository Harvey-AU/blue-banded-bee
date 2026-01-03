-- Fix vault functions for atomicity and cleanup consistency
-- 1. store_webflow_token: Create new secret before deleting old (prevents token loss on partial failure)
-- 2. delete_webflow_token: Clear vault_secret_name column to prevent orphaned references

-- Recreate store_webflow_token with atomic pattern (create-then-delete)
CREATE OR REPLACE FUNCTION store_webflow_token(connection_id UUID, token TEXT)
RETURNS TEXT AS $$
DECLARE
  secret_name TEXT;
  old_secret_name TEXT;
BEGIN
  secret_name := 'webflow_token_' || connection_id::TEXT;

  -- Check for existing secret name
  SELECT vault_secret_name INTO old_secret_name
  FROM webflow_connections
  WHERE id = connection_id;

  -- Create new secret first (safer - if this fails, old token still exists)
  PERFORM vault.create_secret(token, secret_name);

  -- Update connection with secret name
  UPDATE webflow_connections
  SET vault_secret_name = secret_name
  WHERE id = connection_id;

  -- Only delete old secret after new one is confirmed
  -- (vault.create_secret would have raised an error if it failed)
  IF old_secret_name IS NOT NULL AND old_secret_name != secret_name THEN
    DELETE FROM vault.secrets WHERE name = old_secret_name;
  ELSIF old_secret_name = secret_name THEN
    -- Same name - delete the old version (create_secret made a new one)
    -- Actually vault.create_secret fails if name exists, so we need upsert logic
    NULL; -- No action needed, handled below
  END IF;

  RETURN secret_name;
EXCEPTION
  WHEN unique_violation THEN
    -- Secret already exists with this name - update it instead
    UPDATE vault.secrets SET secret = token WHERE name = secret_name;
    UPDATE webflow_connections SET vault_secret_name = secret_name WHERE id = connection_id;
    RETURN secret_name;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, vault;

-- Recreate delete_webflow_token to also clear the reference column
CREATE OR REPLACE FUNCTION delete_webflow_token(connection_id UUID)
RETURNS VOID AS $$
DECLARE
  secret_name TEXT;
BEGIN
  secret_name := 'webflow_token_' || connection_id::TEXT;

  -- Clear the reference in webflow_connections first
  UPDATE webflow_connections
  SET vault_secret_name = NULL
  WHERE id = connection_id;

  -- Then delete from vault
  DELETE FROM vault.secrets WHERE name = secret_name;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, vault;
