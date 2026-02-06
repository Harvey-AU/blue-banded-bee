-- Add vault functions for Google Analytics accounts
-- These mirror the connection-level functions but work with the accounts table
-- Accounts store shared tokens that can be reused for refreshing account lists

-- Store a refresh token for a GA account in Supabase Vault
CREATE OR REPLACE FUNCTION store_ga_account_token(account_id UUID, refresh_token TEXT)
RETURNS TEXT AS $$
DECLARE
  secret_name TEXT;
  secret_updated INT;
  account_updated INT;
  account_org_id UUID;
  caller_org_id UUID;
BEGIN
  SELECT organisation_id INTO account_org_id
  FROM google_analytics_accounts
  WHERE id = account_id;

  IF account_org_id IS NULL THEN
    RAISE EXCEPTION 'Account % not found', account_id;
  END IF;

  IF auth.role() <> 'service_role' THEN
    SELECT organisation_id INTO caller_org_id
    FROM users
    WHERE id = auth.uid();

    IF caller_org_id IS NULL OR caller_org_id <> account_org_id THEN
      RAISE EXCEPTION 'Not authorised to access account %', account_id;
    END IF;
  END IF;

  secret_name := 'ga_account_token_' || account_id::TEXT;

  -- Try to update existing secret first (safe - no data loss if it fails)
  UPDATE vault.secrets SET secret = refresh_token WHERE name = secret_name;
  GET DIAGNOSTICS secret_updated = ROW_COUNT;

  -- If no existing secret, create new one
  IF secret_updated = 0 THEN
    PERFORM vault.create_secret(refresh_token, secret_name);
  END IF;

  -- Update account with secret name
  UPDATE google_analytics_accounts
  SET vault_secret_name = secret_name
  WHERE id = account_id;

  -- Verify the account was updated (catches invalid account_id)
  GET DIAGNOSTICS account_updated = ROW_COUNT;
  IF account_updated = 0 THEN
    -- Only clean up if WE created the secret (not if it already existed)
    IF secret_updated = 0 THEN
      DELETE FROM vault.secrets WHERE name = secret_name;
    END IF;
    RAISE EXCEPTION 'Account % not found', account_id;
  END IF;

  RETURN secret_name;
EXCEPTION
  WHEN unique_violation THEN
    -- Race condition: another call created the secret between our UPDATE and CREATE
    -- Retry as update
    UPDATE vault.secrets SET secret = refresh_token WHERE name = secret_name;
    UPDATE google_analytics_accounts SET vault_secret_name = secret_name WHERE id = account_id;
    GET DIAGNOSTICS account_updated = ROW_COUNT;
    IF account_updated = 0 THEN
      RAISE EXCEPTION 'Account % not found', account_id;
    END IF;
    RETURN secret_name;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, vault;

-- Retrieve a refresh token for a GA account from Supabase Vault
CREATE OR REPLACE FUNCTION get_ga_account_token(account_id UUID)
RETURNS TEXT AS $$
DECLARE
  secret_name TEXT;
  token TEXT;
  account_org_id UUID;
  caller_org_id UUID;
BEGIN
  -- First get the secret name from the account
  SELECT organisation_id, vault_secret_name INTO account_org_id, secret_name
  FROM google_analytics_accounts
  WHERE id = account_id;

  IF account_org_id IS NULL THEN
    RAISE EXCEPTION 'Account % not found', account_id;
  END IF;

  IF auth.role() <> 'service_role' THEN
    SELECT organisation_id INTO caller_org_id
    FROM users
    WHERE id = auth.uid();

    IF caller_org_id IS NULL OR caller_org_id <> account_org_id THEN
      RAISE EXCEPTION 'Not authorised to access account %', account_id;
    END IF;
  END IF;

  IF secret_name IS NULL THEN
    RETURN NULL;
  END IF;

  -- Get the decrypted token from vault
  SELECT decrypted_secret INTO token
  FROM vault.decrypted_secrets
  WHERE name = secret_name;

  RETURN token;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, vault;

-- Delete a refresh token for a GA account from Supabase Vault
CREATE OR REPLACE FUNCTION delete_ga_account_token(account_id UUID)
RETURNS VOID AS $$
DECLARE
  secret_name TEXT;
  account_org_id UUID;
  caller_org_id UUID;
BEGIN
  SELECT organisation_id, vault_secret_name INTO account_org_id, secret_name
  FROM google_analytics_accounts
  WHERE id = account_id;

  IF account_org_id IS NULL THEN
    RAISE EXCEPTION 'Account % not found', account_id;
  END IF;

  IF auth.role() <> 'service_role' THEN
    SELECT organisation_id INTO caller_org_id
    FROM users
    WHERE id = auth.uid();

    IF caller_org_id IS NULL OR caller_org_id <> account_org_id THEN
      RAISE EXCEPTION 'Not authorised to access account %', account_id;
    END IF;
  END IF;

  -- Clear the reference in google_analytics_accounts first
  UPDATE google_analytics_accounts
  SET vault_secret_name = NULL
  WHERE id = account_id;

  -- Then delete from vault
  IF secret_name IS NOT NULL THEN
    DELETE FROM vault.secrets WHERE name = secret_name;
  END IF;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, vault;

-- Grant execute permissions to authenticated users
GRANT EXECUTE ON FUNCTION store_ga_account_token(UUID, TEXT) TO authenticated;
GRANT EXECUTE ON FUNCTION get_ga_account_token(UUID) TO authenticated;
GRANT EXECUTE ON FUNCTION delete_ga_account_token(UUID) TO authenticated;

-- Grant execute to service_role for API access
GRANT EXECUTE ON FUNCTION store_ga_account_token(UUID, TEXT) TO service_role;
GRANT EXECUTE ON FUNCTION get_ga_account_token(UUID) TO service_role;
GRANT EXECUTE ON FUNCTION delete_ga_account_token(UUID) TO service_role;
