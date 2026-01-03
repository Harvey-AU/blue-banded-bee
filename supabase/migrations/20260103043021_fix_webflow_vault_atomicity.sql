-- Fix vault functions for atomicity and cleanup consistency
-- 1. store_webflow_token: Use upsert pattern with proper error handling
-- 2. delete_webflow_token: Clear vault_secret_name column to prevent orphaned references

-- Recreate store_webflow_token with clean upsert logic
CREATE OR REPLACE FUNCTION store_webflow_token(connection_id UUID, token TEXT)
RETURNS TEXT AS $$
DECLARE
  secret_name TEXT;
  rows_updated INT;
BEGIN
  secret_name := 'webflow_token_' || connection_id::TEXT;

  -- Try to update existing secret first (safe - no data loss if it fails)
  UPDATE vault.secrets SET secret = token WHERE name = secret_name;
  GET DIAGNOSTICS rows_updated = ROW_COUNT;

  -- If no existing secret, create new one
  IF rows_updated = 0 THEN
    PERFORM vault.create_secret(token, secret_name);
  END IF;

  -- Update connection with secret name
  UPDATE webflow_connections
  SET vault_secret_name = secret_name
  WHERE id = connection_id;

  -- Verify the connection was updated (catches invalid connection_id)
  GET DIAGNOSTICS rows_updated = ROW_COUNT;
  IF rows_updated = 0 THEN
    -- Clean up the secret we just created since connection doesn't exist
    DELETE FROM vault.secrets WHERE name = secret_name;
    RAISE EXCEPTION 'Connection % not found', connection_id;
  END IF;

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
