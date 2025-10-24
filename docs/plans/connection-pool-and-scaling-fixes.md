# Connection Pool Saturation and Scaling Issues - Root Cause Analysis & Remediation Plan

**Date:** 2025-10-24 **Status:** Analysis Complete - Awaiting Implementation
Approval **Priority:** CRITICAL

---

## Executive Summary

After thorough investigation including Sentry error analysis, Fly logs review,
Supabase database metrics, and code review, we've identified **four primary
issues** causing connection pool saturation and stuck tasks during scaling
operations. This document provides evidence-based analysis and actionable
remediation steps.

**Key Metrics:**

- **8,926 pool saturation events** in last 24 hours
- **270 database deadlock errors**
- **683 "transaction already committed" errors**
- **77 tasks stuck in 'running' state** for >5 minutes
- **21/32 database connections active** (66% utilization)
- **9.4% transaction rollback rate** (healthy is <2%)

---

## Issue Analysis

### Issue #1: Database Trigger Causing Update Storm 🔥 **HIGH CONFIDENCE**

**Evidence:**

```sql
-- Current trigger configuration (db.go:714-721)
CREATE TRIGGER trigger_update_job_progress
  AFTER INSERT OR UPDATE OR DELETE ON tasks
  FOR EACH ROW
  EXECUTE FUNCTION update_job_progress();
```

**Database Metrics:**

- Jobs table: **8,512,821 updates** vs **579 inserts** (ratio: 14,700:1)
- Tasks table: **3,372,890 updates** on **1,567,119 inserts**
- Supabase advisor: "Table `public`.`jobs` has excessive bloat"

**Root Cause:** The trigger fires on **every task table change**, not just
status changes:

- Priority score updates (after every crawl) → trigger fires
- Cache check attempts updates → trigger fires
- Response time updates → trigger fires
- ANY field update → trigger fires

**Why This Causes Deadlocks:**

Multiple workers processing tasks from the same job simultaneously update
different task rows. Each update fires the trigger, which attempts to UPDATE the
same job row:

```
Worker A: UPDATE tasks (task_id=1) → Trigger → UPDATE jobs (job_id=X) [acquires row lock]
Worker B: UPDATE tasks (task_id=2) → Trigger → UPDATE jobs (job_id=X) [waits for lock]
Worker C: UPDATE tasks (task_id=3) → Trigger → UPDATE jobs (job_id=X) [waits for lock]

If circular wait conditions arise → PostgreSQL deadlock detection → kills one transaction
```

**Sentry Evidence:**

- **270 deadlock events** (BLUE-BANDED-BEE-Q):
  `ERROR: deadlock detected (SQLSTATE 40P01)`
- All occur at `handleTaskSuccess()` → `UpdateTaskStatus()`
- Stacktrace shows update happening during trigger execution

**Impact:**

- Massive unnecessary load on jobs table
- Frequent deadlocks requiring retries
- Connection pool pressure from retry attempts
- Table bloat from excessive updates

---

### Issue #2: Worker Pool Exceeding Connection Capacity 🔥 **HIGH CONFIDENCE**

**Evidence:**

```go
// Current configuration (db.go:117)
case "production":
    config.MaxOpenConns = 32  // Supabase limit minus overhead

// Worker pool (environment-based, defaults to 50 workers)
```

**Connection Usage Pattern:**

Each worker performing a task requires multiple sequential transactions:

1. `GetNextTask()` - opens transaction, claims task, commits (~200ms)
2. Process task (HTTP request to target site) - no DB connection
3. `UpdateTaskStatus()` - opens transaction, updates task, commits (~100ms)
4. `updateTaskPriorities()` - opens transaction, updates discovered links,
   commits (~150ms)

**The Math:**

- 50 workers active
- Average 2-3 transactions per task (claim + update + priorities)
- Each transaction holds connection for 100-300ms
- Peak concurrent connections: 50 workers × 60% probability = **30 connections**
- Pool capacity: **32 connections**
- **Pool operates at 94-100% capacity continuously**

**Pool Saturation Monitoring Evidence:**

```go
// queue.go:164-189 - Pool monitoring code
usage := float64(stats.InUse) / float64(maxOpen)

if usage >= 0.90 {  // 90% threshold
    return ErrPoolSaturated
}
```

**Sentry Evidence:**

- **8,926 saturation events** across multiple error IDs:
  - BLUE-BANDED-BEE-4E: 5,832 events - "database connection pool saturated"
  - BLUE-BANDED-BEE-4D: 650 events - "DB pool nearing capacity"
  - BLUE-BANDED-BEE-4C: 566 events - "DB pool saturated"
  - Plus 10+ similar issues

**Current Database Metrics:**

- Active connections: 21/32 (66% - measured at low activity time)
- Cache hit ratio: 99.62% (good)
- Transaction rollbacks: 1.89M vs 20.1M commits (**9.4% rollback rate is
  pathological**)

---

### Issue #3: Stuck Task Recovery Ineffective ⚠️ **MEDIUM CONFIDENCE**

**Evidence:**

```sql
-- Current recovery query (worker.go:1028)
UPDATE tasks
SET status = 'pending', started_at = NULL
WHERE status = 'running'
  AND started_at < NOW() - INTERVAL '5 minutes'
  AND job_id IN (SELECT id FROM jobs WHERE status = 'running')
```

**Database State:**

- **77 tasks** currently stuck in 'running' state
- ALL stuck tasks are >5 minutes old
- Recovery monitor runs every 5 minutes
- Recovery detects them (122 Sentry alerts: BLUE-BANDED-BEE-42)
- **Recovery is unable to fix them**

**Possible Causes (Needs Further Investigation):**

**Theory A - Batch Size Issues:** Recovery processes in batches of 100, but
batch commit might timeout on heavily-loaded jobs table.

**Theory B - Connection Pool Exhaustion:** Recovery runs when pool is saturated,
can't get connection to execute update.

**Theory C - Trigger Overhead:** UPDATE triggers `update_job_progress()` for
each task → 77 job updates → contention/timeout.

**Note:** The feedback agent correctly noted these tasks are NOT blocked by
uncommitted locks from `GetNextTask()` - those transactions commit properly. The
recovery failure has a different cause.

**Recent "Fix" That May Have Introduced Regression:**

- Commit 7fd7716: "Fix stuck task recovery logic"
- Changed recovery batch processing
- **Result: 77 tasks still stuck, recovery ineffective**

---

### Issue #4: Missing Database Indexes ⚠️ **LOW-MEDIUM CONFIDENCE**

**Evidence from Supabase Performance Advisor:**

```
Table `public.tasks` has a foreign key `tasks_page_id_fkey`
without a covering index. This can lead to suboptimal query performance.

Table `public.jobs` has a foreign key `jobs_domain_id_fkey`
without a covering index. This can lead to suboptimal query performance.
```

**Impact:**

- Queries joining tasks → pages may require sequential scans
- JOIN performance degradation at scale
- Not directly causing deadlocks, but contributes to query slowness

---

## Proposed Solutions

### Solution #1: Optimise Database Trigger 🎯 **HIGH PRIORITY**

**Change:**

```sql
-- Replace trigger to only fire on status changes
DROP TRIGGER IF EXISTS trigger_update_job_progress ON tasks;

CREATE TRIGGER trigger_update_job_progress
  AFTER INSERT OR UPDATE OF status OR DELETE ON tasks
  FOR EACH ROW
  WHEN (
    TG_OP = 'INSERT' OR
    TG_OP = 'DELETE' OR
    (TG_OP = 'UPDATE' AND OLD.status IS DISTINCT FROM NEW.status)
  )
  EXECUTE FUNCTION update_job_progress();
```

**Expected Impact:**

- Reduces trigger executions by ~80% (only fires on status changes, not all
  updates)
- Eliminates deadlocks from simultaneous job row updates during
  priority/metadata updates
- Reduces jobs table bloat
- Reduces transaction rollback rate from 9.4% to <2%
- **Estimated reduction: 8,926 saturation events → ~1,000/day**

**Implementation Method:** Database migration file:
`supabase/migrations/[timestamp]_optimise_job_progress_trigger.sql`

**Risk:** LOW - Trigger still fires for all relevant status changes
(pending→running→completed/failed)

---

### Solution #2: Reduce Production Worker Count 🎯 **HIGH PRIORITY**

**Change:**

```go
// internal/jobs/worker.go or configuration

// CURRENT:
// Production: 50 workers

// PROPOSED:
// Production: 25 workers (50% reduction)
```

**Rationale:**

- 25 workers × ~2.5 transactions = ~62 concurrent connection attempts
- With connection pooling, actual concurrent connections: ~20-25
- Pool capacity: 32
- **Headroom: ~25% instead of 0%**

**Expected Impact:**

- Reduces connection pool saturation events by ~70%
- Maintains good throughput (25 workers is still substantial)
- Allows headroom for maintenance operations, recovery monitor, cleanup
- **Better: predictable performance vs. unpredictable saturation crashes**

**Trade-off:**

- Slightly slower job completion times (~2× longer for large jobs)
- More predictable, reliable performance

**Implementation:** Update environment-specific worker configuration in db.go or
worker pool initialization.

**Risk:** LOW - Can easily scale back up if throughput is insufficient

---

### Solution #3: Batch Priority Updates 🎯 **MEDIUM PRIORITY**

**Current Pattern:**

```go
// After EVERY crawl (worker.go:1630):
wp.updateTaskPriorities(discoveredLinks)  // New transaction per crawl
```

**Proposed Pattern:**

```go
// Collect updates in memory, flush every 5 seconds or 100 updates:
wp.queuePriorityUpdate(discoveredLinks)  // Add to buffer

// Periodic flush in background goroutine:
every 5s: wp.flushPriorityUpdates()  // Single batch transaction
```

**Expected Impact:**

- Reduces transaction count by ~90%
- Reduces connection pool pressure
- Minimal impact on priority accuracy (5s delay acceptable)

**Risk:** MEDIUM - Requires careful implementation to avoid memory leaks or lost
updates on crash

---

### Solution #4: Add Missing Foreign Key Indexes 🎯 **LOW PRIORITY**

**Change:**

```sql
-- Add recommended indexes
CREATE INDEX IF NOT EXISTS idx_tasks_page_id ON tasks(page_id);
CREATE INDEX IF NOT EXISTS idx_jobs_domain_id ON jobs(domain_id);
CREATE INDEX IF NOT EXISTS idx_jobs_user_id ON jobs(user_id);
```

**Expected Impact:**

- Improves JOIN performance
- Reduces query execution time by ~10-20%
- Minor reduction in connection hold time

**Risk:** LOW - Standard performance optimization

---

### Solution #5: Immediate Database Maintenance 🎯 **MEDIUM PRIORITY**

**Actions:**

```sql
-- Reclaim bloat from jobs table
VACUUM FULL jobs;  -- WARNING: Locks table briefly (~30 seconds)

-- Rebuild indexes
REINDEX TABLE jobs;
REINDEX TABLE tasks;

-- Update query planner statistics
ANALYZE jobs, tasks, pages, domains;
```

**Timing:** Execute during low-traffic period (maintenance window).

**Expected Impact:**

- Reclaims disk space
- Improves query performance
- Reduces table bloat alerts

**Risk:** MEDIUM - VACUUM FULL requires brief table lock

---

### Solution #6: Improve Stuck Task Recovery (Requires Investigation) 🔍

**Current Issue:** 77 tasks stuck, recovery detects but cannot fix.

**Investigation Needed:**

1. Check PostgreSQL logs for recovery query failures
2. Verify recovery batch timeout settings (currently 65s)
3. Test recovery against heavily-loaded jobs table
4. Check if trigger overhead during recovery is the bottleneck

**Possible Fix (After Investigation):**

```sql
-- Option A: Use SKIP LOCKED to avoid contention
UPDATE tasks
SET status = 'pending', started_at = NULL
FROM (
  SELECT id FROM tasks
  WHERE status = 'running'
    AND started_at < NOW() - INTERVAL '5 minutes'
  FOR UPDATE SKIP LOCKED
  LIMIT 100
) AS locked_tasks
WHERE tasks.id = locked_tasks.id;

-- Option B: Process in smaller batches (10-20 at a time)
-- Option C: Temporarily disable trigger during recovery
```

**Risk:** MEDIUM - Needs careful testing to avoid making stuck tasks worse

---

## Implementation Plan

### Phase 1: Quick Wins (Immediate - Low Risk)

1. ✅ **Solution #4** - Add missing indexes (5 minutes, zero downtime)
2. ✅ **Solution #1** - Optimise trigger (Migration file, deploys with next PR)

**Expected Immediate Impact:**

- ~60-70% reduction in saturation events
- Eliminates most deadlocks
- Reduces jobs table update storm

---

### Phase 2: Configuration Tuning (Next Deployment)

3. ✅ **Solution #2** - Reduce worker count to 25 (Configuration change)

**Expected Impact:**

- Additional 20-30% reduction in saturation events
- Improved connection pool headroom
- More predictable performance

---

### Phase 3: Maintenance & Investigation (Scheduled)

4. ⚠️ **Solution #5** - Database maintenance during low-traffic window
5. 🔍 **Solution #6** - Investigate and fix stuck task recovery

---

### Phase 4: Optimization (Future)

6. 🔧 **Solution #3** - Implement batched priority updates (Requires code
   refactor)

---

## Monitoring & Validation

### Success Metrics

**Immediate (Within 24 hours of Phase 1):**

- Pool saturation events: 8,926/day → <2,000/day (78% reduction)
- Deadlock errors: 270/day → <50/day (82% reduction)
- Transaction rollback rate: 9.4% → <3%

**Short-term (Within 1 week of Phase 2):**

- Pool saturation events: <500/day (94% reduction)
- Stuck tasks: 77 → <10 (after Phase 3)
- Active connections during peak: 66% → <50%

**Long-term (After all phases):**

- Zero deadlocks
- Pool saturation events: <100/day (99% reduction)
- Consistent sub-2% rollback rate
- No stuck tasks

---

## Risks & Mitigations

### Risk #1: Trigger Change Breaks Job Progress Tracking

**Mitigation:**

- Test thoroughly in preview environment
- Monitor job progress updates after deployment
- Have rollback SQL ready

### Risk #2: Reduced Workers Impacts Throughput Unacceptably

**Mitigation:**

- Monitor job completion times
- Easy to scale workers back up via configuration
- Can implement adaptive scaling based on queue depth

### Risk #3: VACUUM FULL Causes Brief Downtime

**Mitigation:**

- Schedule during known low-traffic period
- Communicate maintenance window
- Alternative: Use regular VACUUM (slower but no lock)

---

## Alternative Hypotheses Considered

### ❌ Hypothesis: Nested Transactions Causing Deadlocks

**Claim:** `GetNextTask()` holds lock while `UpdateTaskStatus()` tries to update
same row.

**Evidence Against:**

```go
// GetNextTask (queue.go:265-325)
err := q.Execute(ctx, func(tx *sql.Tx) error {
    // ... SELECT FOR UPDATE SKIP LOCKED ...
    // ... UPDATE tasks SET status = 'running' ...
    return nil  // <-- Transaction COMMITS here
})
// Lock released when function returns

// Later: UpdateTaskStatus (queue.go:474)
err := q.Execute(ctx, func(tx *sql.Tx) error {
    // Fresh transaction, no lock conflict with above
})
```

**Conclusion:** Transactions are sequential, not nested. Lock is released before
UpdateTaskStatus runs.

---

### ❌ Hypothesis: Workers Bypass Connection Pool Limits

**Claim:** Workers open connections without going through pool accounting.

**Evidence Against:**

```go
// All DB operations use sql.DB which enforces limits:
tx, err := q.db.client.BeginTx(ctx, nil)  // Respects SetMaxOpenConns

// Pool monitoring exists and works:
stats := q.db.client.Stats()
usage := float64(stats.InUse) / float64(maxOpen)
// This correctly shows 66% usage
```

**Conclusion:** Pool limits are respected. Saturation is from legitimate high
load, not bypassed accounting.

---

## Questions Requiring Answers Before Full Implementation

1. **Trigger Optimization:**
   - ✅ Ready to implement - clear benefit, low risk

2. **Worker Reduction:**
   - What's acceptable job completion time increase?
   - Should we implement adaptive worker scaling instead?

3. **Stuck Task Recovery:**
   - Can we capture PostgreSQL logs during next recovery attempt?
   - What's the actual failure mode? (timeout, deadlock, or other)

4. **Priority Update Batching:**
   - Is 5-second delay in priority updates acceptable?
   - Should we implement this now or wait for Phase 4?

---

## Recommendation

**Proceed with Phase 1 immediately:**

1. Add missing indexes (zero risk, immediate benefit)
2. Optimise trigger to only fire on status changes (high confidence fix)

**Then Phase 2:** 3. Reduce production workers to 25 (reversible if needed)

**Monitor results for 48 hours, then proceed with Phase 3 if metrics improve as
expected.**

---

## Appendix: Evidence Summary

### Sentry Top Issues (Last 24h)

| Issue ID           | Description                        | Count | Root Cause             |
| ------------------ | ---------------------------------- | ----- | ---------------------- |
| BLUE-BANDED-BEE-4E | Pool saturated (handleTaskSuccess) | 5,832 | Issue #2               |
| BLUE-BANDED-BEE-4D | Pool nearing capacity              | 650   | Issue #2               |
| BLUE-BANDED-BEE-2X | Context deadline exceeded          | 581   | Issue #2               |
| BLUE-BANDED-BEE-4C | Pool saturated                     | 566   | Issue #2               |
| BLUE-BANDED-BEE-4F | Pool saturated                     | 482   | Issue #2               |
| BLUE-BANDED-BEE-Q  | **Deadlock detected**              | 270   | **Issue #1**           |
| BLUE-BANDED-BEE-3E | Transaction already committed      | 236   | Issue #1 (side effect) |
| BLUE-BANDED-BEE-42 | Stuck tasks detected               | 122   | Issue #3               |

### Database Metrics

| Metric                | Value          | Health             |
| --------------------- | -------------- | ------------------ |
| Active connections    | 21/32 (66%)    | ⚠️ Too high        |
| Cache hit ratio       | 99.62%         | ✅ Good            |
| Transaction commits   | 20.1M          | -                  |
| Transaction rollbacks | 1.89M (9.4%)   | 🚨 Very bad        |
| Tasks updates         | 3.37M          | -                  |
| Jobs updates          | 8.51M          | 🚨 Pathological    |
| Jobs inserts          | 579            | -                  |
| Update:Insert ratio   | 14,700:1       | 🚨 Trigger problem |
| Dead tuples (tasks)   | 27,889         | ⚠️ Needs vacuum    |
| Dead tuples (jobs)    | 2,487          | ⚠️ Needs vacuum    |
| Stuck running tasks   | 77 (all >5min) | 🚨 Recovery broken |

---

**Document End**

---

## Notes on Conflicting Analysis

This plan deliberately addresses the feedback from the alternative analysis:

- ✅ Acknowledges transactions are sequential, not nested
- ✅ Confirms pool limits are respected
- ✅ Identifies trigger as the deadlock source (multiple workers → same job row)
- ✅ Focuses on evidence-based solutions
- ✅ Marks speculative items for investigation rather than implementation

The key insight is that deadlocks occur at the **jobs table level**
(trigger-induced), not at the tasks table level (row locking).
