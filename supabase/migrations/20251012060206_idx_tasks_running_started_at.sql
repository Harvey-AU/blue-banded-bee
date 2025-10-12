-- Add an index to accelerate lookups of running tasks by started_at
-- This satisfies Supabase index advisor recommendations for queries like:
--   SELECT ... FROM tasks WHERE status = 'running' AND started_at < ...

CREATE INDEX IF NOT EXISTS idx_tasks_running_started_at
    ON public.tasks (started_at)
    WHERE status = 'running';
