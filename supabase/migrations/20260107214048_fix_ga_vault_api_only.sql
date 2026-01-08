-- Fix GA vault functions to avoid direct table access
-- Use only Vault API functions which have proper permissions on preview branches
--
-- Issue: Direct UPDATE/DELETE on vault.secrets fails with "permission denied"
-- Solution: Use vault.create_secret() only, delete via API if available

-- Rewrite store_ga_token to use delete-then-create pattern
-- This avoids direct UPDATE which fails on preview branches
CREATE OR REPLACE FUNCTION store_ga_token(connection_id UUID, refresh_token TEXT)
RETURNS TEXT AS $$
DECLARE
  secret_name TEXT;
  existing_secret_id UUID;
BEGIN
  secret_name := 'ga_token_' || connection_id::TEXT;

  -- Check if secret exists by looking up its ID
  -- This SELECT is allowed even when UPDATE/DELETE are restricted
  SELECT id INTO existing_secret_id
  FROM vault.secrets
  WHERE name = secret_name;

  IF existing_secret_id IS NOT NULL THEN
    -- Secret exists - use vault.update_secret() API function
    -- This is the proper Vault API way to update secrets
    PERFORM vault.update_secret(existing_secret_id, refresh_token, secret_name, NULL);
  ELSE
    -- Create new secret using vault API function
    PERFORM vault.create_secret(refresh_token, secret_name);
  END IF;

  -- Update connection with secret name
  UPDATE google_analytics_connections
  SET vault_secret_name = secret_name
  WHERE id = connection_id;

  RETURN secret_name;
EXCEPTION
  WHEN unique_violation THEN
    -- Race condition: another call created the secret
    -- Look up the ID and update
    SELECT id INTO existing_secret_id
    FROM vault.secrets
    WHERE name = secret_name;

    IF existing_secret_id IS NOT NULL THEN
      PERFORM vault.update_secret(existing_secret_id, refresh_token, secret_name, NULL);
    END IF;

    UPDATE google_analytics_connections
    SET vault_secret_name = secret_name
    WHERE id = connection_id;

    RETURN secret_name;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, vault;

-- Ensure proper ownership
ALTER FUNCTION store_ga_token(UUID, TEXT) OWNER TO postgres;
