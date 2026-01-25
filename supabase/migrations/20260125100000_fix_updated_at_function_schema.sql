-- Fix update_updated_at_column function schema for Supabase preview branches
-- The original function in 20251229160000_add_slack_integration.sql didn't specify
-- explicit schema, causing "permission denied for schema pg_catalog" on preview branches.

CREATE OR REPLACE FUNCTION public.update_updated_at_column()
RETURNS TRIGGER
LANGUAGE plpgsql
SET search_path = public
AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$;
