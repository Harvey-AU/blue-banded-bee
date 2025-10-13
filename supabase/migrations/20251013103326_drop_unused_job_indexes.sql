-- Drop unused indexes on jobs table
-- These indexes were created for performance tracking (PR #120) but are never used in queries
-- All three columns (stats, avg_time_per_task_seconds, duration_seconds) are only SELECTed,
-- never filtered (WHERE) or sorted (ORDER BY), so the indexes provide zero benefit.
-- Verified by: pg_stat_user_indexes (0 scans) and codebase audit (no WHERE/ORDER BY usage)
-- Saves ~1.3 MB of index storage overhead

-- Drop GIN index on JSONB stats column (496 kB, 0 scans)
DROP INDEX IF EXISTS idx_jobs_stats;

-- Drop B-tree index on avg_time_per_task_seconds (496 kB, 0 scans)
DROP INDEX IF EXISTS idx_jobs_avg_time;

-- Drop B-tree index on duration_seconds (280 kB, 0 scans)
DROP INDEX IF EXISTS idx_jobs_duration;

-- Note: Columns remain intact - only indexes are removed
-- Indexes can be recreated instantly when/if future features need to query these columns
