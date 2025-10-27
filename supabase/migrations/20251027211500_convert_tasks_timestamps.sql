-- Convert the high-traffic tasks table in isolation to minimise lock contention.
-- Doing this in its own migration prevents deadlocks when other sessions touch jobs/tasks.

ALTER TABLE tasks
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN started_at TYPE TIMESTAMPTZ USING started_at AT TIME ZONE 'UTC',
  ALTER COLUMN completed_at TYPE TIMESTAMPTZ USING completed_at AT TIME ZONE 'UTC';
