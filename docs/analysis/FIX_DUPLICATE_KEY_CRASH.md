# Fix for Duplicate Key Constraint Crashes

## Problem
The app was crashing when multiple workers discovered the same links and tried to create duplicate tasks. The error `duplicate key value violates unique constraint "idx_tasks_job_page_unique"` would cascade into app failure.

## Root Cause
Version 0.5.15 centralised task creation through JobManager but didn't implement proper duplicate checking. Multiple workers racing to insert tasks for the same job_id/page_id combination caused constraint violations.

## Solution Implemented

### 1. Modified INSERT Query
Changed the task insertion in `internal/db/queue.go` to use PostgreSQL's `ON CONFLICT DO NOTHING`:

```sql
INSERT INTO tasks (...)
VALUES (...)
ON CONFLICT (job_id, page_id) DO NOTHING
```

This makes duplicate attempts silently succeed rather than error.

### 2. Added Missing Constraint
Added the unique constraint to schema setup in `internal/db/db.go`:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_job_page_unique ON tasks(job_id, page_id)
```

This ensures development/test environments match production.

### 3. Error Handling
With `ON CONFLICT DO NOTHING`, duplicates are no longer errors. The existing error handling now only catches real database errors.

## Testing
Created `internal/db/queue_test.go` with tests that verify:
- Duplicate inserts don't cause errors
- Only one task is created per job/page combination
- Mixed new/duplicate inserts work correctly

## Result
The system is now resilient to concurrent workers discovering the same links. Duplicate attempts are expected behaviour and handled gracefully at the database level.