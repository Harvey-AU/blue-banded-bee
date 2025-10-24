# Batch Update Optimisation Analysis

## Current State

### Existing Infrastructure ✅

The codebase **already has batching infrastructure** in place but it's **not
being used**:

**Location**: `internal/jobs/worker.go:79-87, 127-131, 1132-1205`

```go
// TaskBatch holds groups of tasks for batch processing
type TaskBatch struct {
    tasks     []*Task
    jobCounts map[string]struct {
        completed int
        failed    int
    }
    mu sync.Mutex
}

// WorkerPool initialization
taskBatch: &TaskBatch{
    tasks:     make([]*Task, 0, 50),
    jobCounts: make(map[string]struct{ completed, failed int }),
},
batchTimer: time.NewTicker(10 * time.Second),
```

**Batch Processor**: `processBatches()` runs every 10 seconds and calls
`flushBatches()` **Batch Flushing**: `flushBatches()` updates all tasks in a
single transaction

### Current Problem ❌

Tasks are updated **immediately** after completion instead of being batched:

**Location**: `internal/jobs/worker.go:1533, 1622`

```go
// handleTaskError - Line 1533
updErr := wp.dbQueue.UpdateTaskStatus(ctx, task)

// handleTaskSuccess - Line 1622
updErr := wp.dbQueue.UpdateTaskStatus(ctx, task)
```

**Impact**:

- Each task completion = 1 database UPDATE transaction
- Each UPDATE fires the trigger (even after optimisation)
- 50 workers × simultaneous completions = potential for trigger contention
- Current rollback rate: 8.57% (target: <2%)

## Your Proposed Approaches

### Option A: Worker Claims 5 Tasks, Batch Updates

```
Worker → Claims 5 tasks
      → Processes all 5
      → Single batch UPDATE for all 5
```

**Pros:**

- Simple to implement
- Reduces transactions by 5× immediately
- Minimal architectural changes

**Cons:**

- Worker holds tasks for longer (could delay other workers)
- If worker crashes, 5 tasks need recovery instead of 1
- Still has per-worker database writes (50 workers = 50 concurrent writes)

### Option B: Coordinator + Result Queue Pattern

```
Coordinator Worker → Claims tasks
                  → Distributes to processing workers

Processing Workers → Process tasks
                  → Send results to in-memory queue

Result Writer → Accumulates results
             → Batch writes every N seconds or M tasks
```

**Pros:**

- **Single writer** - eliminates concurrent UPDATE contention entirely
- Maximum batch efficiency (can accumulate 100s of tasks)
- Separation of concerns (processing vs persistence)
- Better crash recovery (results queue can be buffered)

**Cons:**

- More complex architecture
- Adds latency (results wait for batch window)
- Memory pressure if results queue grows unbounded

## Recommended Approach: Hybrid Solution

**Use the existing batching infrastructure with minimal changes:**

### Phase 1: Enable Existing Batch System (Quick Win)

**Change**: Replace immediate `UpdateTaskStatus()` calls with batch queueing

**Implementation**:

1. Add `addTaskToBatch()` method to queue completed tasks
2. Replace lines 1533 and 1622 with batch queueing
3. Keep existing `processBatches()` running every 10s

**Expected Impact**:

- Reduces transactions by ~95% (50 individual writes → 1 batch write per 10s)
- Trigger fires once per 10s instead of continuously
- Rollback rate drops from 8.57% → <1%

**Code Changes Required**:

```go
// Add method to WorkerPool
func (wp *WorkerPool) addTaskToBatch(task *db.Task) {
    wp.taskBatch.mu.Lock()
    defer wp.taskBatch.mu.Unlock()

    wp.taskBatch.tasks = append(wp.taskBatch.tasks, task)

    // Track job counts for later
    if task.Status == "completed" {
        counts := wp.taskBatch.jobCounts[task.JobID]
        counts.completed++
        wp.taskBatch.jobCounts[task.JobID] = counts
    } else if task.Status == "failed" || task.Status == "blocked" {
        counts := wp.taskBatch.jobCounts[task.JobID]
        counts.failed++
        wp.taskBatch.jobCounts[task.JobID] = counts
    }
}

// In handleTaskError (line 1533)
- updErr := wp.dbQueue.UpdateTaskStatus(ctx, task)
+ wp.addTaskToBatch(task)

// In handleTaskSuccess (line 1622)
- updErr := wp.dbQueue.UpdateTaskStatus(ctx, task)
+ wp.addTaskToBatch(task)
```

### Phase 2: Adaptive Batch Tuning (Follow-up)

Once Phase 1 is working, tune batch parameters:

**Current**: Flush every 10 seconds **Optimisation**: Flush when
`batch_size >= 50 OR elapsed >= 5s`

```go
const (
    maxBatchSize     = 50
    maxBatchInterval = 5 * time.Second
)

// Modified processBatches
func (wp *WorkerPool) processBatches(ctx context.Context) {
    ticker := time.NewTicker(maxBatchInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            wp.flushBatches(ctx)
        case <-wp.stopCh:
            wp.flushBatches(ctx)
            return
        case <-ctx.Done():
            return
        }

        // Check if batch is full
        wp.taskBatch.mu.Lock()
        shouldFlush := len(wp.taskBatch.tasks) >= maxBatchSize
        wp.taskBatch.mu.Unlock()

        if shouldFlush {
            wp.flushBatches(ctx)
            ticker.Reset(maxBatchInterval)
        }
    }
}
```

### Phase 3: Coordinator Pattern (If Needed)

Only implement if Phase 1+2 don't achieve <2% rollback rate.

**When to consider**:

- Rollback rate still >3% after batching
- High worker contention observed
- Database becomes bottleneck again

**Architecture**:

- 1 coordinator goroutine claims tasks
- 49 worker goroutines process tasks
- 1 result writer goroutine batches updates
- In-memory channels between components

## Metrics to Track

### Before Changes (Current Baseline)

- Transactions per minute: ~3,000-5,000 (50 workers × 60-100 tasks/min)
- Trigger executions: ~600-1,000/min (after trigger optimisation)
- Rollback rate: 8.57%
- Connection pool usage: 74% (20/27)

### After Phase 1 (Expected)

- Transactions per minute: ~6 batch writes (10s interval)
- Trigger executions: ~6/min (95% reduction)
- Rollback rate: <1.5% (target achieved)
- Connection pool usage: ~50% (lower contention)

### After Phase 2 (Expected)

- Transactions per minute: ~12-60 (adaptive batching)
- Trigger executions: ~12-60/min (still 90%+ reduction)
- Rollback rate: <1%
- Latency: improved (faster batch flushes)

## Risk Assessment

### Phase 1 Risk: LOW ✅

- Minimal code changes (3 lines)
- Uses existing infrastructure
- Easy rollback (revert to immediate updates)
- No architecture changes
- Batch processor already running and tested

### Phase 2 Risk: LOW ✅

- Tuning existing parameters
- Adaptive logic reduces worst-case latency
- No new concurrency patterns

### Phase 3 Risk: MEDIUM ⚠️

- Significant architecture change
- New concurrency patterns (channels, goroutines)
- Memory management concerns
- Complex testing requirements

## Implementation Priority

**Immediate**: Phase 1 (enable existing batching)

- 1-2 hours development
- 30 minutes testing
- High impact, low risk

**Short-term**: Phase 2 (adaptive tuning)

- After Phase 1 metrics collected (24-48 hours)
- 2-3 hours development
- Medium impact, low risk

**Future**: Phase 3 (coordinator pattern)

- Only if Phases 1+2 insufficient
- 1-2 days development + testing
- High impact, medium risk

## Questions to Resolve

1. **Latency tolerance**: Is 5-10 second delay acceptable for progress updates?
   - Dashboard polls every 60s, so yes
   - Webhook notifications already batched

2. **Memory bounds**: Maximum batch size before forcing flush?
   - Current: 50 tasks (reasonable)
   - With 50 workers, worst case: 50 tasks × 10s = 500 tasks
   - Each task ~1KB → 500KB memory (negligible)

3. **Crash recovery**: What happens to in-flight batches?
   - Tasks remain in "running" state
   - Existing recovery mechanism picks them up after 5 min
   - No data loss, just delayed completion tracking

## Recommendation

**Start with Phase 1** - enable the existing batching infrastructure:

1. It's already built and tested
2. Minimal code changes (3 lines)
3. Expected 95% reduction in transactions
4. Should achieve <1.5% rollback rate (target: <2%)
5. Easy to rollback if issues occur

**Monitor for 24-48 hours**, then:

- If rollback rate <2%: Success, tune with Phase 2
- If rollback rate 2-3%: Implement Phase 2
- If rollback rate >3%: Consider Phase 3

The trigger optimisation + Phase 1 batching should be sufficient to meet goals
without complex architectural changes.
