-- Fix GA vault functions for atomicity (matching Webflow pattern from 20260103043021)
-- 1. store_ga_token: Use upsert pattern with proper error handling
-- 2. delete_ga_token: Clear vault_secret_name column to prevent orphaned references

-- Recreate store_ga_token with clean upsert logic
CREATE OR REPLACE FUNCTION store_ga_token(connection_id UUID, refresh_token TEXT)
RETURNS TEXT AS $$
DECLARE
  secret_name TEXT;
  secret_updated INT;
  connection_updated INT;
BEGIN
  secret_name := 'ga_token_' || connection_id::TEXT;

  -- Try to update existing secret first (safe - no data loss if it fails)
  UPDATE vault.secrets SET secret = refresh_token WHERE name = secret_name;
  GET DIAGNOSTICS secret_updated = ROW_COUNT;

  -- If no existing secret, create new one
  IF secret_updated = 0 THEN
    PERFORM vault.create_secret(refresh_token, secret_name);
  END IF;

  -- Update connection with secret name
  UPDATE google_analytics_connections
  SET vault_secret_name = secret_name
  WHERE id = connection_id;

  -- Verify the connection was updated (catches invalid connection_id)
  GET DIAGNOSTICS connection_updated = ROW_COUNT;
  IF connection_updated = 0 THEN
    -- Only clean up if WE created the secret (not if it already existed)
    IF secret_updated = 0 THEN
      DELETE FROM vault.secrets WHERE name = secret_name;
    END IF;
    RAISE EXCEPTION 'Connection % not found', connection_id;
  END IF;

  RETURN secret_name;
EXCEPTION
  WHEN unique_violation THEN
    -- Race condition: another call created the secret between our UPDATE and CREATE
    -- Retry as update
    UPDATE vault.secrets SET secret = refresh_token WHERE name = secret_name;
    UPDATE google_analytics_connections SET vault_secret_name = secret_name WHERE id = connection_id;
    GET DIAGNOSTICS connection_updated = ROW_COUNT;
    IF connection_updated = 0 THEN
      RAISE EXCEPTION 'Connection % not found', connection_id;
    END IF;
    RETURN secret_name;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, vault;

-- Recreate delete_ga_token to also clear the reference column
CREATE OR REPLACE FUNCTION delete_ga_token(connection_id UUID)
RETURNS VOID AS $$
DECLARE
  secret_name TEXT;
BEGIN
  secret_name := 'ga_token_' || connection_id::TEXT;

  -- Clear the reference in google_analytics_connections first
  UPDATE google_analytics_connections
  SET vault_secret_name = NULL
  WHERE id = connection_id;

  -- Then delete from vault
  DELETE FROM vault.secrets WHERE name = secret_name;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, vault;
