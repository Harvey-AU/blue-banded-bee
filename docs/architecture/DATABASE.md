# Database Reference

## Overview

Blue Banded Bee uses PostgreSQL as its primary database with a normalised schema designed for efficiency and data integrity. The system leverages PostgreSQL-specific features like `FOR UPDATE SKIP LOCKED` for lock-free concurrent task processing.

As of 26th July 2025 we manage database schema/setup via migrations.

### Migration Workflow

Blue Banded Bee uses Supabase GitHub integration for automatic migration deployment:

1. **Create Migration Files**: Place new `.sql` files in `supabase/migrations/` with timestamp prefix
2. **Push to GitHub**: Migrations apply automatically when merged to `test-branch` or `main`
3. **No Manual Steps**: Supabase handles all migration execution via GitHub integration

**Important**: Do NOT run `supabase db push` manually - let the GitHub integration handle it.

## Connection Configuration

### Environment Variables

```bash
# Method 1: Single URL (recommended for production)
DATABASE_URL="postgres://user:password@host:port/database?sslmode=require"

# Method 2: Individual components (useful for development)
DB_HOST=localhost
DB_PORT=5432
DB_USER=your_user
DB_PASSWORD=your_password
DB_NAME=bluebandedbee
DB_SSLMODE=prefer
```

### Connection Pool Settings

Optimised for high-concurrency workloads:

```go
// Located in internal/db/db.go
client.SetMaxOpenConns(25)      // Maximum open connections
client.SetMaxIdleConns(10)      // Maximum idle connections
client.SetConnMaxLifetime(5 * time.Minute)  // Connection lifetime
client.SetConnMaxIdleTime(2 * time.Minute)  // Idle connection timeout
```

## Database Schema

### Core Tables

#### Domains Table

Stores unique domain names with integer primary keys for normalisation.

```sql
CREATE TABLE domains (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_domains_name ON domains(name);
```

#### Pages Table

Stores page paths with domain references to reduce redundancy.

```sql
CREATE TABLE pages (
    id SERIAL PRIMARY KEY,
    domain_id INTEGER REFERENCES domains(id),
    path TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(domain_id, path)
);

CREATE INDEX idx_pages_domain_path ON pages(domain_id, path);
```

#### Jobs Table

Stores job metadata and progress tracking.

```sql
CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    domain_id INTEGER REFERENCES domains(id),
    user_id TEXT,
    organisation_id TEXT,
    status TEXT NOT NULL,
    progress REAL DEFAULT 0.0,
    total_tasks INTEGER DEFAULT 0,
    completed_tasks INTEGER DEFAULT 0,
    failed_tasks INTEGER DEFAULT 0,
    skipped_tasks INTEGER DEFAULT 0,
    found_tasks INTEGER DEFAULT 0,
    sitemap_tasks INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    concurrency INTEGER DEFAULT 1,
    find_links BOOLEAN DEFAULT FALSE,
    max_pages INTEGER DEFAULT 100,
    include_paths TEXT,
    exclude_paths TEXT,
    required_workers INTEGER DEFAULT 1
);

-- Indexes for performance
CREATE INDEX idx_jobs_status ON jobs(status);
CREATE INDEX idx_jobs_user_org ON jobs(user_id, organisation_id);
CREATE INDEX idx_jobs_created_at ON jobs(created_at);
```

**Status Values:**

- `pending` - Job created but not started
- `running` - Job actively processing tasks
- `completed` - All tasks finished successfully
- `cancelled` - Job manually cancelled
- `failed` - Job failed due to system error

**Task Counters:**

- `total_tasks` = `sitemap_tasks` + `found_tasks`
- `sitemap_tasks` - URLs from sitemap processing
- `found_tasks` - URLs discovered through link crawling
- `completed_tasks` - Successfully processed URLs
- `failed_tasks` - URLs that failed processing
- `skipped_tasks` - URLs skipped due to limits or filters

#### Tasks Table

Stores individual URL processing tasks.

```sql
CREATE TABLE tasks (
    id TEXT PRIMARY KEY,
    job_id TEXT REFERENCES jobs(id),
    domain_id INTEGER REFERENCES domains(id),
    page_id INTEGER REFERENCES pages(id),
    status TEXT NOT NULL,
    source_type TEXT,
    source_url TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    status_code INTEGER,
    response_time INTEGER,
    cache_status TEXT,
    content_type TEXT,
    error TEXT,
    retry_count INTEGER DEFAULT 0
);

-- Indexes for task processing
CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_job_id ON tasks(job_id);
CREATE INDEX idx_tasks_job_status ON tasks(job_id, status);
CREATE INDEX idx_tasks_pending ON tasks(created_at) WHERE status = 'pending';
```

**Status Values:**

- `pending` - Task waiting to be processed
- `running` - Task currently being processed
- `completed` - Task successfully completed
- `failed` - Task failed after retries
- `skipped` - Task skipped due to limits

### Authentication Tables

#### Users Table

Extends Supabase auth.users with application-specific data.

```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY, -- Matches Supabase auth.users(id)
    email TEXT NOT NULL UNIQUE,
    full_name TEXT,
    organisation_id TEXT REFERENCES organisations(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_organisation ON users(organisation_id);
```

#### Organisations Table

Simple organisation model for data sharing.

```sql
CREATE TABLE organisations (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
```

## PostgreSQL-Specific Features

### Lock-Free Task Processing

Uses `FOR UPDATE SKIP LOCKED` to allow multiple workers to claim tasks without blocking:

```sql
-- Task acquisition query (internal/db/queue.go)
SELECT t.id, t.job_id, d.name as domain, p.path, t.source_type, t.source_url
FROM tasks t
JOIN pages p ON t.page_id = p.id
JOIN domains d ON p.domain_id = d.id
WHERE t.job_id = ANY($1)
  AND t.status = 'pending'
ORDER BY t.created_at ASC
LIMIT 1
FOR UPDATE SKIP LOCKED;
```

### Batch Operations

Efficient bulk inserts using PostgreSQL arrays:

```sql
-- Batch task creation (internal/db/queue.go)
INSERT INTO tasks (id, job_id, domain_id, page_id, status, source_type, source_url, created_at)
SELECT
    gen_random_uuid()::text,
    $1,
    $2,
    unnest($3::integer[]),
    'pending',
    $4,
    $5,
    NOW()
```

### Progress Tracking

Atomic progress updates with conditional status changes:

```sql
-- Job progress update (internal/db/queue.go)
UPDATE jobs
SET
    progress = CASE
        WHEN total_tasks > 0 THEN
            ((completed_tasks + failed_tasks)::REAL / total_tasks::REAL) * 100.0
        ELSE 0.0
    END,
    completed_tasks = (
        SELECT COUNT(*) FROM tasks
        WHERE job_id = $1 AND status = 'completed'
    ),
    failed_tasks = (
        SELECT COUNT(*) FROM tasks
        WHERE job_id = $1 AND status = 'failed'
    )
WHERE id = $1;
```

## Database Operations

### URL Processing Strategy

Instead of storing full URLs, the system:

1. **Normalises domains** into the `domains` table
2. **Stores paths** in the `pages` table with domain references
3. **References pages** from `tasks` table
4. **Reconstructs URLs** by joining domain + path during processing

Benefits:

- Reduces storage redundancy
- Improves data integrity
- Enables efficient domain-based queries
- Supports domain-level analytics

### Task Lifecycle

```sql
-- 1. Task Creation
INSERT INTO tasks (id, job_id, domain_id, page_id, status, ...)
VALUES (gen_random_uuid()::text, ?, ?, ?, 'pending', ...);

-- 2. Task Claiming (atomic)
UPDATE tasks SET
    status = 'running',
    started_at = NOW()
WHERE id = ? AND status = 'pending';

-- 3. Task Completion
UPDATE tasks SET
    status = 'completed',
    completed_at = NOW(),
    status_code = ?,
    response_time = ?,
    cache_status = ?
WHERE id = ?;
```

### Recovery Operations

Handles system restarts and stuck jobs:

```sql
-- Reset stuck running tasks on startup
UPDATE tasks
SET status = 'pending',
    started_at = NULL,
    retry_count = retry_count + 1
WHERE status = 'running'
  AND started_at < NOW() - INTERVAL '10 minutes';

-- Mark jobs complete when all tasks finished
UPDATE jobs
SET status = 'completed',
    completed_at = NOW(),
    progress = 100.0
WHERE status IN ('pending', 'running')
  AND total_tasks > 0
  AND total_tasks = completed_tasks + failed_tasks;
```

## Row Level Security (RLS)

### User Data Isolation

```sql
-- Enable RLS on user tables
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE tasks ENABLE ROW LEVEL SECURITY;

-- Users can only access their own data
CREATE POLICY "users_own_data" ON users
FOR ALL USING (auth.uid()::text = id);

-- Organisation members can access shared jobs
CREATE POLICY "org_jobs_access" ON jobs
FOR ALL USING (
    organisation_id IN (
        SELECT organisation_id FROM users
        WHERE id = auth.uid()::text
    )
);

-- Tasks inherit job access permissions
CREATE POLICY "job_tasks_access" ON tasks
FOR ALL USING (
    job_id IN (
        SELECT id FROM jobs
        WHERE organisation_id IN (
            SELECT organisation_id FROM users
            WHERE id = auth.uid()::text
        )
    )
);
```

## Performance Optimisation

### Query Optimisation

**Indexed Queries:**

- Task status lookups use `idx_tasks_status`
- Job lookups by user/org use `idx_jobs_user_org`
- Pending task queries use partial index `idx_tasks_pending`

**Connection Management:**

- Connection pooling reduces overhead
- Long-lived connections for worker processes
- Automatic reconnection on failures

**Batch Processing:**

- Bulk inserts for task creation
- Batch updates for progress tracking
- Efficient array operations for URL processing

### Monitoring Queries

```sql
-- Check worker efficiency
SELECT
    status,
    COUNT(*) as count,
    AVG(EXTRACT(EPOCH FROM (completed_at - started_at))) as avg_duration
FROM tasks
WHERE started_at > NOW() - INTERVAL '1 hour'
GROUP BY status;

-- Monitor job completion rates
SELECT
    DATE_TRUNC('hour', created_at) as hour,
    COUNT(*) as jobs_created,
    COUNT(completed_at) as jobs_completed
FROM jobs
WHERE created_at > NOW() - INTERVAL '24 hours'
GROUP BY hour
ORDER BY hour;

-- Check database performance
SELECT
    schemaname,
    tablename,
    seq_scan,
    seq_tup_read,
    idx_scan,
    idx_tup_fetch
FROM pg_stat_user_tables
WHERE schemaname = 'public';
```

## Migration Management

### Creating Migrations

1. **Generate migration file**:

   ```bash
   supabase migration new your_migration_name
   ```

   This creates a timestamped file in `supabase/migrations/`

2. **Write migration SQL**:

   ```sql
   -- Add new columns safely
   ALTER TABLE jobs
   ADD COLUMN IF NOT EXISTS new_field TEXT DEFAULT '';

   -- Create indexes concurrently (non-blocking)
   CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_new_field
   ON jobs(new_field);

   -- Remove old columns after confirming unused
   ALTER TABLE jobs DROP COLUMN IF EXISTS old_field;
   ```

3. **Test locally** (optional):

   ```bash
   supabase start
   supabase db reset  # Applies all migrations
   ```

4. **Deploy via GitHub**:
   - Push to feature branch
   - Create PR to `test-branch` (migrations auto-apply)
   - After testing, merge to `main` (migrations auto-apply)

### Migration Files

All migrations are in `supabase/migrations/`:

- `20240101000000_initial_schema.sql` - Base schema creation
- `20250720013915_remote_schema.sql` - Initial remote state
- `20250727212804_add_job_duration_fields.sql` - Calculated duration fields
- New migrations get timestamped names automatically

## Backup & Recovery

### Backup Strategy

```bash
# Full database backup
pg_dump -h host -U user -d database > backup_$(date +%Y%m%d_%H%M%S).sql

# Schema-only backup
pg_dump -h host -U user -d database --schema-only > schema_backup.sql

# Data-only backup
pg_dump -h host -U user -d database --data-only > data_backup.sql
```

### Recovery Testing

```bash
# Restore from backup
psql -h host -U user -d new_database < backup_file.sql

# Verify data integrity
SELECT COUNT(*) FROM jobs;
SELECT COUNT(*) FROM tasks;
SELECT COUNT(*) FROM users;
```

## Known Issues & Solutions

### Schema Evolution Problems

**Issue**: `CREATE TABLE IF NOT EXISTS` doesn't modify existing tables
**Solution**: Use explicit `ALTER TABLE` commands for schema changes

**Issue**: Column removal requires data migration
**Solution**:

1. Add new columns first
2. Migrate data
3. Remove old columns in separate deployment

### Performance Gotchas

**Issue**: Sequential scans on large task tables
**Solution**: Ensure proper indexing on status and job_id columns

**Issue**: Connection pool exhaustion under high load
**Solution**: Monitor active connections and tune pool settings

**Issue**: Lock contention on job progress updates
**Solution**: Use atomic updates and avoid frequent progress writes

## Development Workflow

### Local Setup

```bash
# Create local database
createdb bluebandedbee

# Set environment variables
export DATABASE_URL="postgres://localhost/bluebandedbee"

# Run application (creates schema automatically)
go run ./cmd/app/main.go
```

### Testing Database

```bash
# Run with test database
export DATABASE_URL="postgres://localhost/bluebandedbee_test"
export RUN_INTEGRATION_TESTS=true
go test ./...
```

### Schema Reset (Development Only)

```bash
# WARNING: Destroys all data
curl -X POST localhost:8080/admin/reset-db \
  -H "Authorization: Bearer admin-token"
```

This database design provides a solid foundation for Blue Banded Bee's cache warming operations while maintaining data integrity, performance, and security.
