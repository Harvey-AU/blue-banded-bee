-- Fix GA delete trigger conflict
-- The cleanup trigger was calling delete_ga_token() which tries to UPDATE the row
-- being deleted, causing "tuple to be deleted was already modified" error
--
-- Solution: Delete directly from vault in the trigger instead of calling the function

CREATE OR REPLACE FUNCTION cleanup_ga_vault_secret()
RETURNS TRIGGER AS $$
DECLARE
  secret_name TEXT;
BEGIN
  -- Build secret name directly (same logic as delete_ga_token)
  secret_name := 'ga_token_' || OLD.id::TEXT;

  -- Delete directly from vault - don't call delete_ga_token() which updates the row
  DELETE FROM vault.secrets WHERE name = secret_name;

  RETURN OLD;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, vault;
