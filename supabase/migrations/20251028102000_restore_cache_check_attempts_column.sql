-- Ensure cache_check_attempts column exists on tasks
-- Table was initially created without this column in earlier deployments,
-- and CREATE TABLE IF NOT EXISTS does not backfill additional fields.

ALTER TABLE tasks
  ADD COLUMN IF NOT EXISTS cache_check_attempts JSONB;

COMMENT ON COLUMN tasks.cache_check_attempts IS 'Array of request attempts (timestamps, cache states) used for cache validation diagnostics.';
