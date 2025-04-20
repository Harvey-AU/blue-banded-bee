# PostgreSQL Configuration Variables

This document lists all the configuration variables for the PostgreSQL database to optimize performance and reduce lock contention.

## Database Schema

The application uses a normalized database schema with reference tables to improve data integrity and reduce redundancy.

### Core Tables

**domains**: Stores unique domain names with integer primary keys
```sql
CREATE TABLE IF NOT EXISTS domains (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
)
```

**pages**: Stores page paths with references to their respective domains
```sql
CREATE TABLE IF NOT EXISTS pages (
    id INTEGER PRIMARY KEY,
    domain_id INTEGER NOT NULL REFERENCES domains(id),
    path TEXT NOT NULL,
    UNIQUE(domain_id, path)
)
```

**jobs**: Stores crawl jobs with references to domains
```sql
CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    domain_id INTEGER NOT NULL REFERENCES domains(id),
    status TEXT NOT NULL,
    -- other fields omitted for brevity
)
```

**tasks**: Stores individual URL crawl tasks with references to pages
```sql
CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL,
    page_id INTEGER NOT NULL REFERENCES pages(id),
    path TEXT NOT NULL,
    status TEXT NOT NULL,
    -- other fields omitted for brevity
)
```

### Important Notes

1. All SQL queries use PostgreSQL-style numbered parameters (`$1`, `$2`, etc.) instead of MySQL/SQLite-style (`?`).
2. When processing tasks, the full URL is reconstructed by joining the domain name from the `domains` table with the path from the `tasks` table.
3. The reference structure ensures data integrity and reduces redundancy by storing domain names and page paths only once.

## Connection Pool Settings

**File**: `internal/db/db.go`

```go
// Current optimized values
client.SetMaxOpenConns(25)
client.SetMaxIdleConns(10)
client.SetConnMaxLifetime(5 * time.Minute)
client.SetConnMaxIdleTime(2 * time.Minute)
```

## Queue Workers

**File**: `internal/db/worker.go`

```go
// Current value
workerPool := db.NewWorkerPool(database, crawler, 3) // 3 concurrent workers
```

## Task Queue Implementation

**File**: `internal/db/queue.go`

```go
// Task selection uses PostgreSQL's FOR UPDATE SKIP LOCKED for efficient concurrent processing
rows, err := tx.QueryContext(ctx, `
    SELECT id, job_id, url, depth, source_type, source_url
    FROM tasks
    WHERE status = 'pending'
    ORDER BY created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
`, args...)
```

## Job Progress Tracking

**File**: `internal/db/queue.go`

```go
// Uses explicit REAL type casting for PostgreSQL
_, err = tx.ExecContext(ctx, `
    UPDATE jobs
    SET
        progress = $1::REAL,
        completed_tasks = $2,
        failed_tasks = $3,
        status = CASE
            WHEN $1::REAL >= 100.0 THEN 'completed'
            ELSE status
        END,
        completed_at = CASE
            WHEN $1::REAL >= 100.0 THEN NOW()
            ELSE completed_at
        END
    WHERE id = $4
`, progress, completed, failed, jobID)
```

## Batch Processing

**File**: `internal/db/queue.go`

```go
// Optimized batch insert using PostgreSQL's unnest for better performance
result, err := tx.ExecContext(ctx, `
    INSERT INTO tasks (id, job_id, url, status, depth, source_type, source_url, created_at)
    SELECT
        gen_random_uuid(),
        $1,
        url,
        'pending',
        $2,
        $3,
        $4,
        NOW()
    FROM unnest($5::text[]) AS url
`, jobID, depth, sourceType, sourceURL, pq.Array(urls))
