-- Fix vault function authorisation
-- Critical security fix: All vault functions must verify the caller owns the connection
-- before allowing access to tokens
--
-- Security model:
-- - service_role: Used by backend API, which validates authorisation before calling
-- - authenticated: Used by direct client access, must validate ownership here
-- When auth.uid() IS NULL, we're running as service_role (backend validates)
-- When auth.uid() IS NOT NULL, we must check organisation membership

-- store_webflow_token: stores access token in vault (with authorisation check)
CREATE OR REPLACE FUNCTION store_webflow_token(connection_id UUID, token TEXT)
RETURNS TEXT AS $$
DECLARE
  secret_name TEXT;
  existing_secret_id UUID;
  conn_org_id UUID;
  caller_uid UUID;
BEGIN
  -- Get the caller's user ID (NULL for service_role)
  caller_uid := auth.uid();

  -- SECURITY: If caller is authenticated user (not service_role), verify ownership
  IF caller_uid IS NOT NULL THEN
    SELECT organisation_id INTO conn_org_id
    FROM webflow_connections
    WHERE id = connection_id;

    IF conn_org_id IS NULL THEN
      RAISE EXCEPTION 'Connection not found';
    END IF;

    IF NOT EXISTS (
      SELECT 1 FROM public.user_organisations() AS org_id
      WHERE org_id = conn_org_id
    ) THEN
      RAISE EXCEPTION 'Access denied: not authorised to modify this connection';
    END IF;
  END IF;

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

-- get_webflow_token: retrieves decrypted token from vault (with authorisation check)
CREATE OR REPLACE FUNCTION get_webflow_token(connection_id UUID)
RETURNS TEXT AS $$
DECLARE
  token TEXT;
  secret_name TEXT;
  conn_org_id UUID;
  caller_uid UUID;
BEGIN
  -- Get the caller's user ID (NULL for service_role)
  caller_uid := auth.uid();

  -- SECURITY: If caller is authenticated user (not service_role), verify ownership
  IF caller_uid IS NOT NULL THEN
    SELECT organisation_id INTO conn_org_id
    FROM webflow_connections
    WHERE id = connection_id;

    IF conn_org_id IS NULL THEN
      RAISE EXCEPTION 'Connection not found';
    END IF;

    IF NOT EXISTS (
      SELECT 1 FROM public.user_organisations() AS org_id
      WHERE org_id = conn_org_id
    ) THEN
      RAISE EXCEPTION 'Access denied: not authorised to read this token';
    END IF;
  END IF;

  secret_name := 'webflow_token_' || connection_id::TEXT;

  SELECT decrypted_secret INTO token
  FROM vault.decrypted_secrets
  WHERE name = secret_name;

  RETURN token;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, vault;

-- delete_webflow_token: removes token from vault (with authorisation check)
CREATE OR REPLACE FUNCTION delete_webflow_token(connection_id UUID)
RETURNS VOID AS $$
DECLARE
  secret_name TEXT;
  conn_org_id UUID;
  caller_uid UUID;
BEGIN
  -- Get the caller's user ID (NULL for service_role)
  caller_uid := auth.uid();

  -- SECURITY: If caller is authenticated user (not service_role), verify ownership
  IF caller_uid IS NOT NULL THEN
    SELECT organisation_id INTO conn_org_id
    FROM webflow_connections
    WHERE id = connection_id;

    IF conn_org_id IS NULL THEN
      -- Connection doesn't exist, nothing to delete
      RETURN;
    END IF;

    IF NOT EXISTS (
      SELECT 1 FROM public.user_organisations() AS org_id
      WHERE org_id = conn_org_id
    ) THEN
      RAISE EXCEPTION 'Access denied: not authorised to delete this token';
    END IF;
  END IF;

  secret_name := 'webflow_token_' || connection_id::TEXT;
  DELETE FROM vault.secrets WHERE name = secret_name;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, vault;

-- Ensure cleanup trigger is properly connected
DROP TRIGGER IF EXISTS webflow_connection_cleanup_trigger ON webflow_connections;
CREATE TRIGGER webflow_connection_cleanup_trigger
  BEFORE DELETE ON webflow_connections
  FOR EACH ROW
  EXECUTE FUNCTION cleanup_webflow_vault_secret();

-- Retain existing grants
GRANT EXECUTE ON FUNCTION store_webflow_token(UUID, TEXT) TO service_role;
GRANT EXECUTE ON FUNCTION get_webflow_token(UUID) TO service_role;
GRANT EXECUTE ON FUNCTION delete_webflow_token(UUID) TO service_role;
GRANT EXECUTE ON FUNCTION store_webflow_token(UUID, TEXT) TO authenticated;
GRANT EXECUTE ON FUNCTION get_webflow_token(UUID) TO authenticated;
GRANT EXECUTE ON FUNCTION delete_webflow_token(UUID) TO authenticated;

COMMENT ON FUNCTION store_webflow_token IS 'Stores Webflow access token securely in vault with authorisation check';
COMMENT ON FUNCTION get_webflow_token IS 'Retrieves Webflow access token from vault with authorisation check';
COMMENT ON FUNCTION delete_webflow_token IS 'Deletes Webflow access token from vault with authorisation check';
