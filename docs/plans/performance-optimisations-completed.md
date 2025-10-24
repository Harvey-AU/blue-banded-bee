# Performance Optimisations - Completed

This document tracks completed performance optimisations that were previously
tracked in planning documents.

## Batch Task Status Updates ✅ (2025-10-24)

**Status:** IMPLEMENTED in PR #141

**Original Goal:** Reduce database transaction load from task status updates

- Source: `supabase-optimise.md` section 5 "Review High-Frequency Updates"
- Target: 20-30% reduction in overall database load

**Actual Implementation:**

- PostgreSQL batch UPDATE system using `unnest()` arrays
- Groups updates by status (completed/failed/skipped/pending)
- Flushes every 5 seconds or 100 tasks
- Error classification (infrastructure vs data errors)
- Poison pill isolation
- Graceful shutdown with retry

**Results:**

- 95% reduction in task UPDATE transactions (3000/min → 60/min)
- Exceeds original 20-30% target by 3x
- Zero data loss guarantee during shutdown

**Files:**

- `internal/db/batch.go` - Core batch manager
- `internal/db/batch_test.go` - Comprehensive tests
- `internal/jobs/worker.go` - Integration

## Connection Pool and Trigger Optimisations ✅ (2025-10-24)

**Status:** IMPLEMENTED in commits 4529e92, 7fd7716

**Original Plan:** `connection-pool-and-scaling-fixes.md`

### Phase 1: Quick Wins ✅

1. **Trigger optimisation** - Fire only on task status changes (not all updates)
   - Result: 80% reduction in trigger executions
   - Eliminated deadlocks from simultaneous job row updates

2. **Missing indexes** - Added foreign key covering indexes
   - `idx_tasks_page_id`
   - `idx_jobs_domain_id`
   - `idx_jobs_user_id`

### Phase 2: Configuration Tuning ✅

3. **Worker count reduction** - Production: 50 → 25 workers
   - Reduced connection pool pressure
   - Maintained throughput with better stability
   - Created ~25% connection pool headroom

**Results:**

- Pool saturation events: 8,926/day → <500/day (94% reduction)
- Deadlock errors: 270/day → ~0 (100% reduction)
- Transaction rollback rate: 9.4% → <2% (78% reduction)

## Outstanding Items

### Phase 3: Maintenance & Investigation (Future)

From `connection-pool-and-scaling-fixes.md`:

1. **Database maintenance** - Operational task, not code
   - VACUUM FULL during low-traffic window
   - REINDEX tables
   - Update query planner statistics

2. **Stuck task recovery investigation** - Monitoring ongoing
   - Current: 77 tasks stuck (recovery detected but ineffective)
   - Requires PostgreSQL log analysis
   - May need SKIP LOCKED approach

## Removed Planning Documents

The following planning documents have been superseded by implementations:

- `docs/plans/supabase-optimise.md` - Batching implemented
- `docs/plans/connection-pool-and-scaling-fixes.md` - Phases 1-2 complete

Consolidated into this completion tracker.
