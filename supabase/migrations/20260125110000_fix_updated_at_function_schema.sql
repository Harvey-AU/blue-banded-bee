-- Fix update_updated_at_column function for Supabase preview branches
-- Original function lacked explicit schema causing pg_catalog permission errors

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
