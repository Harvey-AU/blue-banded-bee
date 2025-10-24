# Performance Optimisation Round 2 - Lock Contention & Indexing

**Status:** 📋 PLANNED **Priority:** HIGH **Created:** 2025-10-24 **Context:**
Load test revealed lock contention and missing indexes, but database
connectivity is healthy

---

## Background

Load testing on 2025-10-24 with 41,893 tasks across 6 jobs revealed:

- ✅ Database connectivity is excellent (no connection errors)
- ✅ System processing correctly at 400-550 tasks/minute
- ⚠️ Lock contention causing 2-6 second waits on task updates
- ⚠️ Missing foreign key indexes causing table scans
- ⚠️ RLS policies re-evaluating auth on every row

**See:**
[docs/metrics/load-test-2025-10-24.md](../metrics/load-test-2025-10-24.md)

---

## Goals

1. Reduce lock contention on task/job updates
2. Add missing foreign key indexes
3. Fix RLS policy performance issues
4. Improve task throughput from 450/min to 800-1000/min

**Expected Impact:**

- 2-3x faster job completion
- Reduced database CPU usage
- Better user experience for large jobs

---

## Plan

### 1. Add Missing Foreign Key Indexes

**Problem:** Queries scanning entire tables without indexes, holding locks
longer

**Solution:** Create indexes on heavily-queried foreign keys

**Files to modify:**

- Create new migration:
  `supabase/migrations/YYYYMMDDHHMMSS_add_foreign_key_indexes.sql`

**Implementation:**

```sql
-- High priority: jobs table lookups
CREATE INDEX CONCURRENTLY idx_jobs_domain_id ON jobs(domain_id);
CREATE INDEX CONCURRENTLY idx_jobs_user_id ON jobs(user_id);
CREATE INDEX CONCURRENTLY idx_jobs_organisation_id ON jobs(organisation_id);

-- Medium priority: task lookups
CREATE INDEX CONCURRENTLY idx_tasks_page_id ON tasks(page_id);
CREATE INDEX CONCURRENTLY idx_tasks_job_id_status ON tasks(job_id, status);

-- Low priority: other tables
CREATE INDEX CONCURRENTLY idx_users_organisation_id ON users(organisation_id);
CREATE INDEX CONCURRENTLY idx_job_share_links_created_by ON job_share_links(created_by);
```

**Why CONCURRENTLY:**

- Doesn't block writes during index creation
- Safe for production
- Takes longer but zero downtime

**Testing:**

```bash
# Apply migration to local/dev
supabase db push

# Check index creation
supabase db execute "
SELECT schemaname, tablename, indexname, indexdef
FROM pg_indexes
WHERE tablename IN ('jobs', 'tasks', 'users', 'job_share_links')
ORDER BY tablename, indexname;
"

# Verify query plans improve
EXPLAIN ANALYZE SELECT * FROM jobs WHERE domain_id = 1;
EXPLAIN ANALYZE SELECT * FROM tasks WHERE job_id = 'xxx' AND status = 'pending';
```

**Rollback:**

```sql
DROP INDEX CONCURRENTLY IF EXISTS idx_jobs_domain_id;
DROP INDEX CONCURRENTLY IF EXISTS idx_jobs_user_id;
DROP INDEX CONCURRENTLY IF EXISTS idx_jobs_organisation_id;
DROP INDEX CONCURRENTLY IF EXISTS idx_tasks_page_id;
DROP INDEX CONCURRENTLY IF EXISTS idx_tasks_job_id_status;
DROP INDEX CONCURRENTLY IF EXISTS idx_users_organisation_id;
DROP INDEX CONCURRENTLY IF EXISTS idx_job_share_links_created_by;
```

**Estimated Impact:** 30-40% reduction in lock wait times

---

### 2. Fix RLS Policy Performance

**Problem:** RLS policies calling `auth.uid()` get re-evaluated for EVERY row,
causing massive overhead

**Current pattern (BAD):**

```sql
CREATE POLICY "Users can access own data"
ON users FOR ALL
USING (id = auth.uid());
```

**Fixed pattern (GOOD):**

```sql
CREATE POLICY "Users can access own data"
ON users FOR ALL
USING (id = (SELECT auth.uid()));
```

**Why this works:**

- `auth.uid()` → evaluated once per row (N times)
- `(SELECT auth.uid())` → evaluated once per query, then cached (1 time)
- At scale: N=10,000 rows = 10,000x reduction in function calls

**Files to modify:**

- Create new migration:
  `supabase/migrations/YYYYMMDDHHMMSS_optimise_rls_policies.sql`

**Policies to fix:**

1. `users` table: "Users can access own data"
2. `organisations` table: "Users can access own organisation"
3. `jobs` table: "Organisation members can access jobs"
4. `tasks` table: "Organisation members can access tasks"

**Implementation:**

```sql
-- 1. Users table
DROP POLICY IF EXISTS "Users can access own data" ON users;
CREATE POLICY "Users can access own data"
ON users FOR ALL
USING (id = (SELECT auth.uid()));

-- 2. Organisations table
DROP POLICY IF EXISTS "Users can access own organisation" ON organisations;
CREATE POLICY "Users can access own organisation"
ON organisations FOR ALL
USING (id = (
  SELECT organisation_id
  FROM users
  WHERE id = (SELECT auth.uid())
));

-- 3. Jobs table
DROP POLICY IF EXISTS "Organisation members can access jobs" ON jobs;
CREATE POLICY "Organisation members can access jobs"
ON jobs FOR ALL
USING (organisation_id = (
  SELECT organisation_id
  FROM users
  WHERE id = (SELECT auth.uid())
));

-- 4. Tasks table
DROP POLICY IF EXISTS "Organisation members can access tasks" ON tasks;
CREATE POLICY "Organisation members can access tasks"
ON tasks FOR ALL
USING (
  EXISTS (
    SELECT 1
    FROM jobs
    WHERE jobs.id = tasks.job_id
      AND jobs.organisation_id = (
        SELECT organisation_id
        FROM users
        WHERE id = (SELECT auth.uid())
      )
  )
);
```

**Testing:**

```bash
# Apply migration
supabase db push

# Check policies
supabase db execute "
SELECT schemaname, tablename, policyname, qual
FROM pg_policies
WHERE tablename IN ('users', 'organisations', 'jobs', 'tasks');
"

# Test that RLS still works
# (as authenticated user)
SELECT * FROM jobs LIMIT 10;  -- Should return only user's jobs
SELECT * FROM tasks LIMIT 10; -- Should return only user's tasks

# Verify performance improvement with EXPLAIN
EXPLAIN ANALYZE SELECT * FROM jobs WHERE status = 'completed';
```

**Documentation:**
https://supabase.com/docs/guides/database/postgres/row-level-security#call-functions-with-select

**Estimated Impact:** 50-70% reduction in query overhead for authenticated
requests

---

### 3. Optimise Task Claiming Logic

**Problem:** Multiple workers competing for same pending tasks, causing
tuple-level locks

**Current behaviour:**

```sql
-- Worker 1, 2, 3 all run this simultaneously
UPDATE tasks
SET status = 'running', started_at = NOW()
WHERE id = 'abc123'
-- Workers wait 2-6 seconds for tuple lock
```

**Solution:** Use `SELECT FOR UPDATE SKIP LOCKED` pattern

**Files to modify:**

- `internal/db/tasks.go` - Update `ClaimTask()` or task fetching logic
- `internal/worker/worker.go` - Ensure using optimised claim method

**Current implementation check:**

```bash
# Search for how we're claiming tasks
grep -n "SELECT.*tasks.*pending" internal/db/*.go
grep -n "UPDATE tasks SET status" internal/db/*.go
```

**Improved pattern:**

```go
// internal/db/tasks.go
func (db *DB) ClaimNextTask(ctx context.Context, workerID string) (*Task, error) {
    var task Task

    // Use SELECT FOR UPDATE SKIP LOCKED to avoid contention
    err := db.pool.QueryRow(ctx, `
        WITH next_task AS (
            SELECT id
            FROM tasks
            WHERE status = 'pending'
            ORDER BY priority_score DESC, created_at ASC
            LIMIT 1
            FOR UPDATE SKIP LOCKED  -- Critical: skip locked rows
        )
        UPDATE tasks
        SET
            status = 'in_progress',
            started_at = NOW()
        FROM next_task
        WHERE tasks.id = next_task.id
        RETURNING tasks.*
    `).Scan(
        &task.ID,
        &task.JobID,
        // ... other fields
    )

    if err == pgx.ErrNoRows {
        return nil, ErrNoTasksAvailable
    }
    if err != nil {
        return nil, fmt.Errorf("failed to claim task: %w", err)
    }

    return &task, nil
}
```

**Why this works:**

- `FOR UPDATE SKIP LOCKED` makes workers skip rows other workers are claiming
- No waiting for locks - worker immediately gets next available task
- Eliminates 2-6 second lock waits seen in logs

**Testing:**

```bash
# Run load test with 3 concurrent jobs
go test -v ./internal/worker -run TestConcurrentTaskClaiming

# Monitor lock waits should drop to near-zero
# Check PostgreSQL logs for "still waiting for" messages
fly logs --app blue-banded-bee | grep "still waiting"

# Should see dramatic reduction in wait events
```

**Estimated Impact:** 80-90% reduction in lock contention

---

## Implementation Order

**Phase 1: Low-Risk Wins (Week 1)**

1. ✅ Add foreign key indexes (30 min)
   - Zero risk, immediate improvement
   - Create migration, push to dev, test, deploy

2. ✅ Fix RLS policies (1 hour)
   - Low risk, massive performance gain
   - Create migration, test auth still works, deploy

**Phase 2: Code Changes (Week 2)** 3. ⚠️ Optimise task claiming logic (2-3
hours)

- Medium risk (changes worker behaviour)
- Implement in `internal/db/tasks.go`
- Add unit tests
- Test with local load
- Deploy and monitor closely

---

## Success Metrics

**Before (Current State):**

- Task throughput: 400-550 tasks/minute
- Lock wait times: 2-6 seconds per update
- Query overhead: RLS evaluating 10,000x per query
- Worker efficiency: ~70% (30% waiting on locks)

**After (Target):**

- Task throughput: 800-1,000 tasks/minute (2x improvement)
- Lock wait times: <100ms per update (20x improvement)
- Query overhead: RLS evaluating 1x per query (10,000x improvement)
- Worker efficiency: ~95% (5% waiting on locks)

**How to measure:**

```sql
-- Check lock waits
SELECT wait_event_type, wait_event, COUNT(*)
FROM pg_stat_activity
WHERE wait_event IS NOT NULL
GROUP BY wait_event_type, wait_event;

-- Check query performance
EXPLAIN ANALYZE SELECT * FROM jobs WHERE organisation_id = 'xxx';

-- Check task throughput
SELECT
  DATE_TRUNC('minute', completed_at) as minute,
  COUNT(*) as tasks_completed
FROM tasks
WHERE completed_at > NOW() - INTERVAL '15 minutes'
GROUP BY minute
ORDER BY minute DESC;
```

---

## Rollback Plan

**If issues occur:**

1. **Indexes causing problems:**

   ```sql
   DROP INDEX CONCURRENTLY idx_jobs_domain_id;
   -- etc...
   ```

2. **RLS policies breaking auth:**
   - Revert migration via Supabase dashboard
   - Or manually restore old policy definitions

3. **Task claiming causing deadlocks:**
   - Revert code changes via git
   - Redeploy previous version
   - Worker pool will return to old behaviour

**Critical:** Always deploy during low-traffic window and monitor for 1-2 hours

---

## Documentation Updates

After implementation, update:

- [ ] `docs/architecture/DATABASE.md` - Document new indexes
- [ ] `docs/architecture/ARCHITECTURE.md` - Update worker pool description
- [ ] `CHANGELOG.md` - Add performance improvement notes
- [ ] `docs/development/DEVELOPMENT.md` - Note RLS best practices

---

## References

- **Supabase RLS Performance:**
  https://supabase.com/docs/guides/database/postgres/row-level-security#call-functions-with-select
- **PostgreSQL Indexing:** https://www.postgresql.org/docs/current/indexes.html
- **SELECT FOR UPDATE SKIP LOCKED:**
  https://www.postgresql.org/docs/current/sql-select.html#SQL-FOR-UPDATE-SHARE
- **Load Test Results:**
  [docs/metrics/load-test-2025-10-24.md](../metrics/load-test-2025-10-24.md)
- **Database Linter Issues:**
  https://supabase.com/docs/guides/database/database-linter

---

## Notes

- All changes use `CONCURRENTLY` for zero-downtime deployment
- RLS fixes are Supabase-recommended best practices
- Task claiming pattern is PostgreSQL advisory locking best practice
- Expected total time: 4-5 hours across 2 weeks
- Risk level: LOW to MEDIUM
- Rollback available for all changes
