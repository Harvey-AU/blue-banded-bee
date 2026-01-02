-- Enable full replica identity for jobs table to ensure Realtime works with RLS
-- This is necessary because the RLS policy filters on organisation_id, 
-- which is not part of the primary key.
alter table public.jobs replica identity full;
