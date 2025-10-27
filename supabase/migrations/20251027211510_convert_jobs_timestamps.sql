-- Convert jobs timestamps after tasks to keep lock acquisition consistent and avoid deadlocks.
-- Generated columns are dropped and recreated so Postgres can rewrite with the new types.

ALTER TABLE jobs
  DROP COLUMN IF EXISTS duration_seconds,
  DROP COLUMN IF EXISTS avg_time_per_task_seconds;

ALTER TABLE jobs
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN started_at TYPE TIMESTAMPTZ USING started_at AT TIME ZONE 'UTC',
  ALTER COLUMN completed_at TYPE TIMESTAMPTZ USING completed_at AT TIME ZONE 'UTC';

ALTER TABLE jobs
  ADD COLUMN duration_seconds INTEGER GENERATED ALWAYS AS (
    CASE
      WHEN started_at IS NOT NULL AND completed_at IS NOT NULL
      THEN EXTRACT(EPOCH FROM (completed_at - started_at))::INTEGER
      ELSE NULL
    END
  ) STORED;

ALTER TABLE jobs
  ADD COLUMN avg_time_per_task_seconds NUMERIC GENERATED ALWAYS AS (
    CASE
      WHEN started_at IS NOT NULL AND completed_at IS NOT NULL AND completed_tasks > 0
      THEN EXTRACT(EPOCH FROM (completed_at - started_at))::NUMERIC / completed_tasks::NUMERIC
      ELSE NULL
    END
  ) STORED;
