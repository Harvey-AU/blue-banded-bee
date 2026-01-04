-- Fix Vault permissions for preview branches (v2)
-- The previous migration silently failed - this version is more explicit
-- and tries multiple approaches to ensure vault access works

-- Approach 1: Grant vault access to service_role directly
-- In Supabase, service_role has elevated privileges
DO $$
BEGIN
  GRANT USAGE ON SCHEMA vault TO service_role;
  GRANT SELECT, INSERT, UPDATE, DELETE ON vault.secrets TO service_role;
  GRANT SELECT ON vault.decrypted_secrets TO service_role;
  RAISE NOTICE 'Granted vault permissions to service_role';
EXCEPTION
  WHEN insufficient_privilege THEN
    RAISE NOTICE 'Could not grant vault permissions to service_role';
  WHEN undefined_table THEN
    RAISE NOTICE 'vault.secrets table does not exist';
END;
$$;

-- Approach 2: Also grant to authenticator (the role used by PostgREST)
DO $$
BEGIN
  GRANT USAGE ON SCHEMA vault TO authenticator;
  GRANT SELECT, INSERT, UPDATE, DELETE ON vault.secrets TO authenticator;
  GRANT SELECT ON vault.decrypted_secrets TO authenticator;
  RAISE NOTICE 'Granted vault permissions to authenticator';
EXCEPTION
  WHEN insufficient_privilege THEN
    RAISE NOTICE 'Could not grant vault permissions to authenticator';
  WHEN undefined_object THEN
    RAISE NOTICE 'authenticator role does not exist';
  WHEN undefined_table THEN
    RAISE NOTICE 'vault.secrets table does not exist';
END;
$$;

-- Recreate Webflow vault functions with explicit owner
-- These functions must be SECURITY DEFINER to access vault with elevated privileges

-- store_webflow_token: stores access token in vault
CREATE OR REPLACE FUNCTION store_webflow_token(connection_id UUID, token TEXT)
RETURNS TEXT AS $$
DECLARE
  secret_name TEXT;
  existing_secret_id UUID;
BEGIN
  secret_name := 'webflow_token_' || connection_id::TEXT;

  -- Check if secret exists first
  SELECT id INTO existing_secret_id
  FROM vault.secrets
  WHERE name = secret_name;

  IF existing_secret_id IS NOT NULL THEN
    -- Update existing secret
    UPDATE vault.secrets
    SET secret = token, updated_at = NOW()
    WHERE id = existing_secret_id;
  ELSE
    -- Create new secret using vault function
    PERFORM vault.create_secret(token, secret_name);
  END IF;

  -- Update connection with secret name
  UPDATE webflow_connections
  SET vault_secret_name = secret_name
  WHERE id = connection_id;

  RETURN secret_name;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, vault;

-- get_webflow_token: retrieves decrypted token from vault
CREATE OR REPLACE FUNCTION get_webflow_token(connection_id UUID)
RETURNS TEXT AS $$
DECLARE
  token TEXT;
  secret_name TEXT;
BEGIN
  secret_name := 'webflow_token_' || connection_id::TEXT;

  SELECT decrypted_secret INTO token
  FROM vault.decrypted_secrets
  WHERE name = secret_name;

  RETURN token;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, vault;

-- delete_webflow_token: removes token from vault
CREATE OR REPLACE FUNCTION delete_webflow_token(connection_id UUID)
RETURNS VOID AS $$
DECLARE
  secret_name TEXT;
BEGIN
  secret_name := 'webflow_token_' || connection_id::TEXT;
  DELETE FROM vault.secrets WHERE name = secret_name;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, vault;

-- Grant execute permissions on these functions
GRANT EXECUTE ON FUNCTION store_webflow_token(UUID, TEXT) TO service_role;
GRANT EXECUTE ON FUNCTION get_webflow_token(UUID) TO service_role;
GRANT EXECUTE ON FUNCTION delete_webflow_token(UUID) TO service_role;
GRANT EXECUTE ON FUNCTION store_webflow_token(UUID, TEXT) TO authenticated;
GRANT EXECUTE ON FUNCTION get_webflow_token(UUID) TO authenticated;
GRANT EXECUTE ON FUNCTION delete_webflow_token(UUID) TO authenticated;
