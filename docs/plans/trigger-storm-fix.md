# Trigger Storm Fix - Job Progress Update Optimization

**Date:** 2025-10-24 **Status:** Ready for Implementation **Priority:** CRITICAL

---

## Problem Summary

The `trigger_update_job_progress` trigger fires on **every task table change**,
not just status changes. This causes:

1. **8.5M job table updates** vs **579 inserts** (14,700:1 ratio)
2. **270 deadlock errors/day** when multiple workers update different tasks from
   same job
3. **Connection pool pressure** from transactions blocking while waiting for job
   row locks
4. **Excessive table bloat** on jobs table (flagged by Supabase advisor)

---

## Evidence

### Database Metrics

```
Jobs table: 8,512,821 updates vs 579 inserts
Tasks table: 3,372,890 updates
```

### Sentry Errors (Last 24h)

- **270 deadlocks:** `ERROR: deadlock detected (SQLSTATE 40P01)` at
  `handleTaskSuccess()`
- **8,926 pool saturation events** across multiple error IDs
- **9.4% transaction rollback rate** (healthy is <2%)

### Current Trigger Configuration

[internal/db/db.go:714-721](../../internal/db/db.go#L714-L721)

```sql
CREATE TRIGGER trigger_update_job_progress
  AFTER INSERT OR UPDATE OR DELETE ON tasks
  FOR EACH ROW
  EXECUTE FUNCTION update_job_progress();
```

**Problem:** Fires on ANY field update, including:

- `priority_score` updates (after every crawl with link discovery)
- `response_time`, `cache_status` updates (every task completion)
- `headers`, `cache_check_attempts` updates (metadata changes)

---

## Root Cause Analysis

### How Deadlocks Occur

```
Worker A: UPDATE tasks SET response_time=... WHERE id='task-1'
          ↓ Trigger fires
          UPDATE jobs SET completed_tasks=... WHERE id='job-x'
          [Acquires row lock on job-x]

Worker B: UPDATE tasks SET cache_status=... WHERE id='task-2'  (same job)
          ↓ Trigger fires
          UPDATE jobs SET completed_tasks=... WHERE id='job-x'
          [BLOCKS waiting for Worker A's lock]

Worker C: UPDATE tasks SET priority_score=... WHERE id='task-3' (same job)
          ↓ Trigger fires
          UPDATE jobs SET completed_tasks=... WHERE id='job-x'
          [BLOCKS waiting for lock]

If circular wait develops → PostgreSQL deadlock detection → kills one transaction
```

### Why Connection Pool Saturates

Each **blocked transaction holds a database connection** while waiting:

- 50 workers × occasional blocking = 10-20 connections held waiting for locks
- Pool capacity: 32 connections
- Remaining capacity for new requests: 12-22 connections
- When burst load hits → pool saturates → rejection threshold (90%) exceeded

---

## Solution: Optimize Trigger to Fire Only on Status Changes

### Implementation

**Create migration:**
`supabase/migrations/[timestamp]_optimise_job_progress_trigger.sql`

```sql
-- Drop existing trigger
DROP TRIGGER IF EXISTS trigger_update_job_progress ON tasks;

-- Recreate trigger with conditions to only fire on status changes
CREATE TRIGGER trigger_update_job_progress
  AFTER INSERT OR UPDATE OF status OR DELETE ON tasks
  FOR EACH ROW
  WHEN (
    TG_OP = 'INSERT' OR
    TG_OP = 'DELETE' OR
    (TG_OP = 'UPDATE' AND OLD.status IS DISTINCT FROM NEW.status)
  )
  EXECUTE FUNCTION update_job_progress();

-- Add comment explaining the optimization
COMMENT ON TRIGGER trigger_update_job_progress ON tasks IS
  'Updates job progress counters only when task status changes.
   Optimized to avoid firing on metadata updates (priority, response_time, etc.)';
```

### Key Changes

1. **`UPDATE OF status`** - Only watch the `status` column, not all columns
2. **`WHEN` clause** - Additional guard to only fire when status actually
   changed
3. **Same function** - No changes to `update_job_progress()` function logic

---

## Expected Impact

### Trigger Execution Reduction

**Current pattern:**

```
Every task completion triggers 3-5 updates:
1. UPDATE status → Trigger fires ✓
2. UPDATE response_time → Trigger fires ✗ (unnecessary)
3. UPDATE priority_score → Trigger fires ✗ (unnecessary)
4. UPDATE cache_status → Trigger fires ✗ (unnecessary)
5. UPDATE headers → Trigger fires ✗ (unnecessary)

= 5 trigger executions per task, 4 unnecessary
```

**After fix:**

```
Only status changes trigger:
1. UPDATE status='running' → Trigger fires ✓
2. UPDATE status='completed' → Trigger fires ✓

= 2 trigger executions per task (80% reduction)
```

### Metrics Improvement Forecast

| Metric                    | Before    | After (Estimated) |
| ------------------------- | --------- | ----------------- |
| Trigger executions        | 3.37M/day | ~670K/day (-80%)  |
| Jobs table updates        | 8.5M/day  | ~1.7M/day (-80%)  |
| Deadlock errors           | 270/day   | <50/day (-82%)    |
| Pool saturation events    | 8,926/day | <1,500/day (-83%) |
| Transaction rollback rate | 9.4%      | <2% (healthy)     |
| Connection pool usage     | 66% avg   | ~40% avg          |

---

## Risks & Mitigations

### Risk 1: Trigger Might Not Fire When Expected

**Concern:** What if we miss a status change?

**Mitigation:**

- The `WHEN` clause is defensive: checks
  `OLD.status IS DISTINCT FROM NEW.status`
- INSERT and DELETE always fire (no OLD/NEW comparison)
- Status transitions are explicit: `pending` → `running` → `completed`/`failed`
- Testing will verify all status transitions fire correctly

### Risk 2: Progress Tracking Becomes Inaccurate

**Concern:** Jobs table might show stale progress.

**Mitigation:**

- Progress is **only meaningful** when status changes anyway
- Metadata updates (priority, response time) don't affect progress calculation
- Function logic unchanged - still counts `completed_tasks` and `failed_tasks`
  accurately
- Existing cleanup job (30s interval) will catch any drift

### Risk 3: Regression in Other Features

**Concern:** Other features might depend on trigger firing frequently.

**Mitigation:**

- Audit shows no features depend on trigger frequency
- Dashboard polls `/v1/jobs` API every 10 seconds (independent of trigger)
- Job completion is calculated from `total_tasks = completed + failed` (still
  works)

---

## Testing Plan

### 1. Preview Environment Testing

Create migration on preview branch → test with real load:

- Create job with 1,000+ tasks
- Verify job progress updates correctly
- Verify all status transitions fire trigger
- Monitor for deadlocks (should be zero)

### 2. Verify Trigger Behavior

```sql
-- Test 1: Status change should fire trigger
UPDATE tasks SET status = 'completed' WHERE id = 'test-task-1';
-- Expected: jobs.completed_tasks increments

-- Test 2: Metadata change should NOT fire trigger
UPDATE tasks SET response_time = 1000 WHERE id = 'test-task-2';
-- Expected: jobs table unchanged

-- Test 3: Combined update (status + metadata) should fire ONCE
UPDATE tasks SET status = 'completed', response_time = 1000 WHERE id = 'test-task-3';
-- Expected: jobs.completed_tasks increments (trigger fires once)
```

### 3. Monitor Production After Deployment

**First 24 hours:**

- Monitor Sentry for deadlock errors (should drop to near-zero)
- Check pool saturation events (should drop ~80%)
- Verify job progress tracking still accurate
- Check transaction rollback rate (<2%)

**If issues occur:**

- Rollback migration immediately
- Investigate which status transition didn't fire
- Fix `WHEN` clause if needed

---

## Implementation Steps

### Step 1: Create Migration

```bash
# Generate migration file
supabase migration new optimise_job_progress_trigger

# Edit file with SQL above
# Commit to feature branch
```

### Step 2: Test in Preview

```bash
# Push branch → creates preview environment
# Preview database automatically applies migration
# Test job creation and progress tracking
# Monitor preview Sentry for errors
```

### Step 3: Merge to Main

```bash
# If preview tests pass:
git push origin feature-branch
# Create PR
# Merge to main
# Migration automatically applies to production
```

### Step 4: Monitor Production

- Watch Sentry dashboard for 24 hours
- Check `/v1/jobs` API responses for accuracy
- Monitor pool saturation metrics
- Verify jobs complete successfully

---

## Rollback Plan

If issues occur in production:

### Immediate Rollback

```sql
-- Revert to original trigger (no conditions)
DROP TRIGGER IF EXISTS trigger_update_job_progress ON tasks;

CREATE TRIGGER trigger_update_job_progress
  AFTER INSERT OR UPDATE OR DELETE ON tasks
  FOR EACH ROW
  EXECUTE FUNCTION update_job_progress();
```

Apply via Supabase SQL editor immediately - no migration needed.

### Long-term Fix

If rollback needed:

1. Investigate which scenario failed (status transition not firing?)
2. Fix `WHEN` clause to handle edge case
3. Create new migration with corrected logic
4. Test again in preview

---

## Alternative Solutions Considered

### Alternative 1: Batch Updates Every 30 Seconds

**Approach:** Disable trigger, update progress via background job.

**Pros:**

- Even more efficient (one update per job per 30s)
- No trigger overhead at all

**Cons:**

- Progress updates lag by up to 30 seconds (poor UX)
- Requires new background process
- More complex implementation
- Higher risk of drift/inaccuracy

**Decision:** Rejected - trigger optimization gives 80% benefit with 5% of the
risk.

---

### Alternative 2: Manual Progress Tracking

**Approach:** Call `update_job_progress()` manually from application code
instead of trigger.

**Pros:**

- Full control over when progress updates
- Can batch multiple status changes

**Cons:**

- Easy to forget to call in new code paths
- Spreads progress logic across codebase
- Higher risk of bugs/missed updates

**Decision:** Rejected - triggers are the right tool for this use case.

---

## Files to Modify

1. ✅ **New file:**
   `supabase/migrations/[timestamp]_optimise_job_progress_trigger.sql`

**No Go code changes required** - purely a database optimization.

---

## Success Criteria

After 24 hours in production:

✅ Deadlock errors: <50/day (currently 270/day) ✅ Pool saturation events:
<1,500/day (currently 8,926/day) ✅ Transaction rollback rate: <2% (currently
9.4%) ✅ Jobs table updates: ~1.7M/day (currently 8.5M/day) ✅ Job progress
tracking: Still accurate (no drift detected) ✅ Dashboard: Shows correct
progress in real-time

---

## Next Steps After This Fix

Once trigger storm is resolved, consider:

1. **Worker pool tuning** - May be able to increase workers from 50 → 60-70 with
   freed capacity
2. **Batching task updates** - Batch 10-20 completed tasks per transaction
   instead of 1-per-transaction
3. **Connection pool monitoring** - Set up alerts for >60% usage
4. **Stuck task recovery** - Fix the 77 stuck tasks (now that recovery won't
   deadlock)

But those are **secondary** - this trigger fix should resolve 80% of scaling
issues.

---

## Approval Required

**Ready to implement?** This is a low-risk, high-impact change:

- ✅ One SQL migration file
- ✅ No Go code changes
- ✅ Easy rollback
- ✅ Tests in preview first
- ✅ Addresses root cause of 270 daily deadlocks

**Recommend proceeding with implementation.**
