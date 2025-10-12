-- Add an index to accelerate lookups of running tasks by started_at
-- This satisfies Supabase index advisor recommendations for queries like:
--   SELECT ... FROM tasks WHERE status = 'running' AND started_at < ...

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_indexes
        WHERE schemaname = 'public'
          AND indexname = 'idx_tasks_running_started_at'
    ) THEN
        CREATE INDEX CONCURRENTLY idx_tasks_running_started_at
            ON public.tasks (started_at)
            WHERE status = 'running';
    END IF;
END
$$;
