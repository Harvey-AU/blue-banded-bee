# Performance Optimisation Summary - Round 2

**Date:** 2025-10-24 **Session Goal:** Scale system from 5 concurrent jobs to
10-50+ concurrent jobs

## Problem Statement

Load testing revealed severe performance bottlenecks preventing the system from
handling even modest concurrent load:

- **Lock Contention:** 24/25 active queries waiting for locks (96% blocked)
- **Slow Task Updates:** 2-6 second wait times per task update
- **Rate Limiting:** 429 errors from Supabase due to database overload
- **Current Capacity:** Only 5 concurrent jobs causing system stress
- **Target Capacity:** 10-50+ concurrent jobs, eventually hundreds
- **RLS Overhead:** Policies causing 10,000x per-row evaluation overhead

## Root Causes Identified

1. **Missing Indexes:** Foreign key columns without indexes causing table scans
2. **RLS Policy Inefficiency:** `auth.uid()` evaluated per-row instead of
   per-query
3. **Task Claiming Logic:** Two-step SELECT + UPDATE holding locks longer than
   necessary

## Solutions Implemented

### 1. Foreign Key Indexes (Migration)

**File:** `supabase/migrations/20251024113314_add_foreign_key_indexes.sql`

Added 9 critical indexes using `CREATE INDEX CONCURRENTLY` for zero-downtime
deployment:

```sql
-- Jobs table (heavily queried)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_jobs_domain_id ON jobs(domain_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_jobs_user_id ON jobs(user_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_jobs_organisation_id ON jobs(organisation_id);

-- Tasks table (high contention)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tasks_job_id_status ON tasks(job_id, status);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tasks_page_id ON tasks(page_id);

-- Partial index for pending task queue
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tasks_status_priority
ON tasks(status, priority_score DESC, created_at ASC)
WHERE status = 'pending';

-- Pages table
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_pages_domain_id ON pages(domain_id);

-- Organisations/users
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_organisation_id ON users(organisation_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_organisations_name ON organisations(name);
```

**Impact:**

- Reduces table scans → faster query execution
- Reduces lock hold time → less contention
- Optimises task queue processing with partial index

### 2. RLS Policy Optimisation (Migration)

**File:** `supabase/migrations/20251024113328_optimise_rls_policies.sql`

Wrapped `auth.uid()` in SELECT to cache result per-query instead of per-row
evaluation:

```sql
-- Before (BAD): Evaluated 10,000x per query
USING (organisation_id = auth.uid())

-- After (GOOD): Evaluated once per query
USING (organisation_id = (SELECT auth.uid()))
```

**Split Policies for Domains/Pages:**

Maintains multi-tenant isolation while allowing worker workflow:

```sql
-- SELECT: Strict - only via jobs in user's organisation
CREATE POLICY "Users can read domains via jobs"
ON domains FOR SELECT
USING (
  EXISTS (
    SELECT 1
    FROM jobs
    WHERE jobs.domain_id = domains.id
      AND jobs.organisation_id = (
        SELECT organisation_id
        FROM users
        WHERE id = (SELECT auth.uid())
      )
  )
);

-- INSERT: Relaxed - workers can create domains before jobs exist
CREATE POLICY "Authenticated users can create domains"
ON domains FOR INSERT
WITH CHECK (auth.role() = 'authenticated');

-- NO UPDATE POLICY: Domains are shared resources
-- Service role only can update to prevent cross-tenant data corruption
```

**Impact:**

- 10,000x reduction in auth function overhead
- Maintains tenant isolation (no cross-organisation data leakage)
- Allows workers to INSERT domains/pages during discovery (service role)
- Users can only SELECT domains/pages via their jobs
- Prevents cross-tenant data corruption (UPDATE restricted to service role only)

### 3. Task Claiming Optimisation (Code)

**File:** `internal/db/queue.go`

Combined SELECT + UPDATE into single CTE query:

```go
// Before: Two queries (SELECT, then UPDATE)
// After: Single atomic CTE query

query := `
    WITH next_task AS (
        SELECT id, job_id, page_id, path, created_at, retry_count,
               source_type, source_url, priority_score
        FROM tasks
        WHERE status = 'pending'
          AND job_id = $2
        ORDER BY priority_score DESC, created_at ASC
        LIMIT 1
        FOR UPDATE SKIP LOCKED
    )
    UPDATE tasks
    SET status = 'running', started_at = $1
    FROM next_task
    WHERE tasks.id = next_task.id
    RETURNING tasks.id, tasks.job_id, tasks.page_id, tasks.path,
              tasks.created_at, tasks.retry_count, tasks.source_type,
              tasks.source_url, tasks.priority_score
`
```

**Impact:**

- 50% fewer database round trips
- Shorter transaction time → less lock holding
- Still uses FOR UPDATE SKIP LOCKED to prevent worker contention

### 4. Bootstrap Code Alignment

**File:** `internal/db/db.go` - `setupRLSPolicies()` function

Updated bootstrap code to match migration optimisations:

- Replaced `DISABLE ROW LEVEL SECURITY` with split policies
- Wrapped all `auth.uid()` calls in SELECT
- Ensures local dev, tests, and ResetSchema() match production behaviour

## Expected Performance Improvements

| Metric               | Before         | After         | Improvement       |
| -------------------- | -------------- | ------------- | ----------------- |
| Lock Wait Time       | 2-6 seconds    | <100ms        | 20x faster        |
| Task Throughput      | 400-550/min    | 800-1,000/min | 2x increase       |
| Query Overhead       | 10,000x        | 1x            | 10,000x reduction |
| Concurrent Jobs      | 5 (struggling) | 10-50+        | 2-10x capacity    |
| Database Round Trips | 2 per claim    | 1 per claim   | 50% reduction     |

## Security Model

The split policy approach maintains strict tenant isolation while preventing
cross-tenant data corruption:

1. **Jobs Table:** Strict RLS on organisation_id (primary enforcement point)
2. **Domains Table (Shared Resource):**
   - **INSERT:** Authenticated users (workers need to create during discovery)
   - **SELECT:** Strict - users only see via their jobs
   - **UPDATE:** Service role only (prevents cross-tenant mutation)
   - **DELETE:** Service role only
3. **Pages Table (Shared Resource):**
   - **INSERT:** Authenticated users (workers discover during crawling)
   - **SELECT:** Strict - users only see via jobs in their organisation
   - **UPDATE:** Service role only (prevents cross-tenant mutation)
   - **DELETE:** Service role only
4. **Tenant Isolation:** Users cannot read domains/pages without jobs in their
   organisation
5. **Data Integrity:** Shared domain/page data cannot be mutated by tenants
   (only service role)
6. **No Cross-Organisation Leakage:** All reads enforced through jobs table RLS

## Files Modified

### New Migrations

1. `supabase/migrations/20251024113314_add_foreign_key_indexes.sql` - 9 indexes
2. `supabase/migrations/20251024113328_optimise_rls_policies.sql` - RLS
   optimisations

### Code Changes

1. `internal/db/queue.go` - CTE-based task claiming (lines 89-128)
2. `internal/db/queue_unit_test.go` - Updated test expectations for new query
   pattern
3. `internal/db/db.go` - setupRLSPolicies() function (lines 800-897)

### Documentation

1. `docs/metrics/load-test-2025-10-24.md` - Load test findings
2. `docs/plans/performance-optimisation-round-2.md` - Implementation plan
3. `docs/optimisations/performance-optimisation-summary.md` - This document

## Testing

All tests passing ✅:

- Unit tests for new CTE query pattern
- Integration tests for task claiming logic
- Schema validation tests
- Mock expectations updated for new query structure

## Deployment Notes

1. **Zero Downtime:** All indexes use `CREATE INDEX CONCURRENTLY`
2. **Automatic Application:** Migrations apply via Supabase GitHub integration
3. **Rollback Safe:** Can roll back migrations if needed
4. **Environment Coverage:** Bootstrap code matches migrations for consistency

## Next Steps

1. Deploy to production via GitHub merge
2. Monitor metrics:
   - Lock wait times (should drop to <100ms)
   - Task throughput (should reach 800-1,000/min)
   - Concurrent job capacity (should handle 10-50+)
   - Database CPU usage (should decrease)
3. Create additional load test jobs to verify scale improvements
4. Observe Sentry for any errors or issues
5. Document production performance gains

## Related Documents

- [Load Test Analysis](../metrics/load-test-2025-10-24.md)
- [Implementation Plan](../plans/performance-optimisation-round-2.md)
- [Database Architecture](../architecture/DATABASE.md)
- [API Reference](../architecture/API.md)
