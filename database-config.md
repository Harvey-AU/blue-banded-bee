# PostgreSQL Configuration Variables

This document lists all the configuration variables for the PostgreSQL database to optimize performance and reduce lock contention.

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
```
