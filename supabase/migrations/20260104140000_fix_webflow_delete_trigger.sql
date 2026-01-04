-- Fix webflow connection delete trigger
-- The current delete_webflow_token function tries to UPDATE the row being deleted,
-- which causes: "tuple to be deleted was already modified by an operation triggered by the current command"
-- 
-- Solution: Create a dedicated cleanup function for the delete trigger that
-- only deletes the vault secret without trying to update the connection row

CREATE OR REPLACE FUNCTION cleanup_webflow_vault_secret()
RETURNS TRIGGER AS $$
DECLARE
  secret_name TEXT;
BEGIN
  -- Only delete the vault secret - don't try to update the row being deleted
  secret_name := 'webflow_token_' || OLD.id::TEXT;
  DELETE FROM vault.secrets WHERE name = secret_name;
  RETURN OLD;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, vault;

-- The trigger already exists, we just replaced the function it calls
