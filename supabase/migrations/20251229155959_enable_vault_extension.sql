-- Enable Vault extension before other migrations that depend on it
-- This runs before the Slack integration migration
-- Handle permission errors gracefully for preview branches

DO $$
BEGIN
  -- Try to enable vault extension if not already enabled
  -- On preview branches, it may already be enabled
  CREATE EXTENSION IF NOT EXISTS supabase_vault WITH SCHEMA vault;
EXCEPTION
  WHEN insufficient_privilege THEN
    RAISE NOTICE 'supabase_vault extension already enabled or insufficient privileges - continuing';
  WHEN duplicate_object THEN
    RAISE NOTICE 'supabase_vault extension already exists - continuing';
  WHEN OTHERS THEN
    RAISE NOTICE 'Could not create supabase_vault extension: % - continuing', SQLERRM;
END;
$$;
