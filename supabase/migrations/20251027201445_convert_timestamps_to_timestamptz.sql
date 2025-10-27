-- Convert top-level tables that don't participate in heavy FK relationships first.
-- Jobs and tasks are handled in follow-up migrations to avoid long-lived locks.

-- Domains table
ALTER TABLE domains
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC';

-- Users table
ALTER TABLE users
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC';

-- Organisations table
ALTER TABLE organisations
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC';

-- Pages table
ALTER TABLE pages
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC';

-- Job share links table
ALTER TABLE job_share_links
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at TYPE TIMESTAMPTZ USING expires_at AT TIME ZONE 'UTC',
  ALTER COLUMN revoked_at TYPE TIMESTAMPTZ USING revoked_at AT TIME ZONE 'UTC';
