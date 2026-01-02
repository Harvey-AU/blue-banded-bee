-- Fix Vault permissions for preview branches
-- Supabase preview branches may have different permission contexts
-- This ensures our SECURITY DEFINER functions can access vault.secrets

-- Grant vault schema permissions to postgres (function owner)
-- These should already exist but we re-grant to ensure preview branches work
DO $$
BEGIN
  -- Ensure vault schema is accessible
  GRANT USAGE ON SCHEMA vault TO postgres;
  GRANT ALL ON ALL TABLES IN SCHEMA vault TO postgres;
  GRANT ALL ON ALL SEQUENCES IN SCHEMA vault TO postgres;
  GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA vault TO postgres;

  -- Also grant to service_role for direct access when needed
  GRANT USAGE ON SCHEMA vault TO service_role;
  GRANT SELECT, INSERT, UPDATE, DELETE ON vault.secrets TO service_role;
EXCEPTION
  WHEN insufficient_privilege THEN
    RAISE NOTICE 'Could not grant vault permissions - may require superuser';
  WHEN undefined_table THEN
    RAISE NOTICE 'vault.secrets table does not exist yet';
END;
$$;

-- Recreate the Webflow vault functions to ensure they have the correct owner
-- Using SET search_path for security (prevents search_path hijacking)
CREATE OR REPLACE FUNCTION store_webflow_token(connection_id UUID, token TEXT)
RETURNS TEXT AS $$
DECLARE
  secret_name TEXT;
BEGIN
  secret_name := 'webflow_token_' || connection_id::TEXT;

  -- Use atomic upsert pattern - delete existing and create new
  DELETE FROM vault.secrets WHERE name = secret_name;
  PERFORM vault.create_secret(token, secret_name);

  -- Update connection with secret name
  UPDATE webflow_connections
  SET vault_secret_name = secret_name
  WHERE id = connection_id;

  RETURN secret_name;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, vault;

-- Recreate get_webflow_token with secure search_path
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

-- Recreate delete_webflow_token with secure search_path
CREATE OR REPLACE FUNCTION delete_webflow_token(connection_id UUID)
RETURNS VOID AS $$
DECLARE
  secret_name TEXT;
BEGIN
  secret_name := 'webflow_token_' || connection_id::TEXT;
  DELETE FROM vault.secrets WHERE name = secret_name;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, vault;
