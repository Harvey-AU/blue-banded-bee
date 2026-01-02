-- Enable postgres_changes realtime for jobs table
-- This allows the frontend to receive real-time updates for job progress and status

-- Add jobs table to the supabase_realtime publication
DO $$
BEGIN
    -- Check if table is already in publication before adding
    IF NOT EXISTS (
        SELECT 1 FROM pg_publication_tables
        WHERE pubname = 'supabase_realtime'
        AND schemaname = 'public'
        AND tablename = 'jobs'
    ) THEN
        ALTER PUBLICATION supabase_realtime ADD TABLE public.jobs;
    END IF;
END $$;
